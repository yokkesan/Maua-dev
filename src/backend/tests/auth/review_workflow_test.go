package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backend/internal/adminauth"
	"backend/internal/notification"
	"backend/internal/passwordreset"
	"backend/internal/review"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func workflowExec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := testDB.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}
func workflowID(t *testing.T, q string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(context.Background(), q, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

var requestSequence atomic.Uint64

func workflowRequest(method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = fmt.Sprintf("[2001:db8::%x]:1234", requestSequence.Add(1))
	r.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)
	return w
}
func workflowStatus(t *testing.T, r *httptest.ResponseRecorder, status int) {
	t.Helper()
	if r.Code != status {
		t.Fatalf("HTTP %d want %d: %s", r.Code, status, r.Body.String())
	}
}
func workflowAdmin(t *testing.T) (int64, *http.Cookie) {
	t.Helper()
	var db string
	if err := testDB.QueryRow(context.Background(), "SELECT current_database()").Scan(&db); err != nil || db != "auma_test" {
		t.Fatal("auma_test required")
	}
	s := adminauth.Service{Repository: &adminauth.Repository{DB: testDB}}
	email := fmt.Sprintf("info-%d@example.com", time.Now().UnixNano())
	if err := s.Create(context.Background(), email, "operator-test-password"); err != nil {
		t.Fatal(err)
	}
	id := workflowID(t, "SELECT id FROM admin_users WHERE email=$1", email)
	t.Cleanup(func() {
		workflowExec(t, "DELETE FROM review_decisions WHERE admin_user_id=$1", id)
		workflowExec(t, "DELETE FROM admin_users WHERE id=$1", id)
	})
	r := workflowRequest("POST", "/api/v1/info/login", fmt.Sprintf(`{"email":%q,"password":"operator-test-password"}`, email))
	workflowStatus(t, r, 200)
	var cookie *http.Cookie
	for _, c := range r.Result().Cookies() {
		if c.Name == adminauth.CookieName {
			cookie = c
		}
	}
	if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1/info" {
		t.Fatal("unsafe operator cookie")
	}
	if strings.Contains(r.Body.String(), "password") || strings.Contains(r.Body.String(), cookie.Value) {
		t.Fatal("secret leaked")
	}
	return id, cookie
}
func workflowUserCookie(t *testing.T, user testUser) *http.Cookie {
	t.Helper()
	r := workflowRequest("POST", "/api/v1/login", fmt.Sprintf(`{"email":%q,"password":%q}`, user.Email, testPassword))
	workflowStatus(t, r, 200)
	for _, c := range r.Result().Cookies() {
		if c.Name == "auma_session" {
			return c
		}
	}
	t.Fatal("missing user cookie")
	return nil
}
func TestInfoAuthenticationIsolationAndExpiry(t *testing.T) {
	_, cookie := workflowAdmin(t)
	user := createTestUser(t, 2)
	general := workflowUserCookie(t, user)
	workflowStatus(t, workflowRequest("GET", "/api/v1/info/me", "", cookie), 200)
	workflowStatus(t, workflowRequest("GET", "/api/v1/info/me", "", general), 401)
	workflowStatus(t, workflowRequest("GET", "/api/v1/me", "", cookie), 401)
	workflowStatus(t, workflowRequest("GET", "/api/v1/me", "", general), 200)
	for _, path := range []string{"/api/v1/info/brands", "/api/v1/info/shops", "/api/v1/info/notification-events", "/api/v1/info/brands/1"} {
		workflowStatus(t, workflowRequest("GET", path, "", general), 401)
	}
	workflowStatus(t, workflowRequest("PATCH", "/api/v1/info/brands/1/review", `{"company_id":1,"approval":1}`, general), 401)
	r := httptest.NewRequest("POST", "/api/v1/info/logout", nil)
	r.Header.Set("Origin", "https://evil.example")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(w, r)
	workflowStatus(t, w, 403)
	workflowStatus(t, workflowRequest("POST", "/api/v1/info/logout", "", cookie), 200)
	workflowStatus(t, workflowRequest("GET", "/api/v1/info/me", "", cookie), 401)
}
func TestInfoSessionExpiration(t *testing.T) {
	_, cookie := workflowAdmin(t)
	h := sha256.Sum256([]byte(cookie.Value))
	workflowExec(t, "UPDATE admin_sessions SET expires_at=NOW()-INTERVAL '1 second' WHERE token_hash=$1", h[:])
	workflowStatus(t, workflowRequest("GET", "/api/v1/info/me", "", cookie), 401)
}
func TestReviewTransitionsAndConcurrency(t *testing.T) {
	adminID, cookie := workflowAdmin(t)
	for _, kind := range []string{"brands", "shops"} {
		t.Run(kind, func(t *testing.T) {
			userType := int16(2)
			if kind == "shops" {
				userType = 1
			}
			user := createTestUser(t, userType)
			var id int64
			if kind == "brands" {
				id = workflowID(t, "SELECT id FROM brands WHERE company_id=$1", user.CompanyID)
			} else {
				id = workflowID(t, "INSERT INTO shops(company_id,name,registration_plan) VALUES ($1,'review shop',10) RETURNING id", user.CompanyID)
			}
			path := fmt.Sprintf("/api/v1/info/%s/%d", kind, id)
			workflowStatus(t, workflowRequest("GET", path, "", cookie), 404)
			body := fmt.Sprintf(`{"company_id":%d,"approval":1}`, user.CompanyID)
			workflowStatus(t, workflowRequest("PATCH", path+"/review", body, cookie), 409)
			workflowExec(t, "UPDATE "+kind+" SET submitted_at=NOW(),status=1 WHERE id=$1", id)
			list := workflowRequest("GET", "/api/v1/info/"+kind, "", cookie)
			workflowStatus(t, list, 200)
			var items struct {
				Items []struct {
					ID int64 `json:"id"`
				}
			}
			if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, i := range items.Items {
				if i.ID == id {
					found = true
				}
			}
			if !found {
				t.Fatal("missing pending target")
			}
			workflowStatus(t, workflowRequest("GET", path, "", cookie), 200)
			workflowStatus(t, workflowRequest("PATCH", path+"/review", fmt.Sprintf(`{"company_id":%d,"approval":1}`, user.CompanyID+99999), cookie), 404)
			for _, approval := range []int{0, 3, -1} {
				workflowStatus(t, workflowRequest("PATCH", path+"/review", fmt.Sprintf(`{"company_id":%d,"approval":%d}`, user.CompanyID, approval), cookie), 400)
			}
			start := make(chan struct{})
			results := make(chan int, 6)
			var wg sync.WaitGroup
			for i := 0; i < 6; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					results <- workflowRequest("PATCH", path+"/review", body, cookie).Code
				}()
			}
			close(start)
			wg.Wait()
			close(results)
			success := 0
			for code := range results {
				if code == 200 {
					success++
				} else if code != 409 {
					t.Fatalf("concurrent HTTP %d", code)
				}
			}
			if success != 1 {
				t.Fatalf("success count %d", success)
			}
			if workflowID(t, "SELECT status FROM "+kind+" WHERE id=$1", id) != 1 {
				t.Fatal("status changed")
			}
			if workflowID(t, "SELECT count(*) FROM review_decisions WHERE admin_user_id=$1 AND entity=$2 AND target_id=$3", adminID, kind, id) != 1 {
				t.Fatal("duplicate review")
			}
			if workflowID(t, "SELECT count(*) FROM notification_events WHERE event_key=$1", fmt.Sprintf("decision:%s:%d", kind, id)) != 1 {
				t.Fatal("duplicate result notification")
			}
			workflowStatus(t, workflowRequest("PATCH", path+"/review", fmt.Sprintf(`{"company_id":%d,"approval":2}`, user.CompanyID), cookie), 409)
			workflowStatus(t, workflowRequest("POST", "/api/v1/login", fmt.Sprintf(`{"email":%q,"password":%q}`, user.Email, testPassword)), 200)
		})
	}
}
func TestReviewRejectionAndMissingManagerRollback(t *testing.T) {
	adminID, _ := workflowAdmin(t)
	u := createTestUser(t, 2)
	id := workflowID(t, "SELECT id FROM brands WHERE company_id=$1", u.CompanyID)
	workflowExec(t, "UPDATE brands SET submitted_at=NOW() WHERE id=$1", id)
	s := review.Service{Repository: &review.Repository{DB: testDB}}
	workflowExec(t, "UPDATE users SET role=1 WHERE id=$1", u.ID)
	if err := s.Decide(context.Background(), "brands", id, u.CompanyID, adminID, 2); err == nil {
		t.Fatal("missing manager accepted")
	}
	if workflowID(t, "SELECT approval FROM brands WHERE id=$1", id) != 0 || workflowID(t, "SELECT count(*) FROM review_decisions WHERE entity='brands' AND target_id=$1", id) != 0 {
		t.Fatal("failed review partially saved")
	}
	workflowExec(t, "UPDATE users SET role=0 WHERE id=$1", u.ID)
	if err := s.Decide(context.Background(), "brands", id, u.CompanyID, adminID, 2); err != nil {
		t.Fatal(err)
	}
	var subject, body, to string
	if err := testDB.QueryRow(context.Background(), "SELECT subject,body,recipient FROM notification_events WHERE event_key=$1", fmt.Sprintf("decision:brands:%d", id)).Scan(&subject, &body, &to); err != nil {
		t.Fatal(err)
	}
	if to != u.Email || !strings.Contains(subject, "完了できませんでした") || !strings.Contains(body, "運営審査が完了できませんでした。") {
		t.Fatal("incorrect rejection mail")
	}
}
func TestGeneralLoginDoesNotAggregateEntityStates(t *testing.T) {
	u := createTestUser(t, 2)
	id := workflowID(t, "SELECT id FROM brands WHERE company_id=$1", u.CompanyID)
	workflowExec(t, "INSERT INTO brands(company_id,name,approval,status) VALUES ($1,'second brand',2,1)", u.CompanyID)
	workflowExec(t, "INSERT INTO shops(company_id,name,registration_plan,approval,status) VALUES ($1,'separate shop',10,2,0)", u.CompanyID)
	for approval := 0; approval <= 2; approval++ {
		for status := 0; status <= 1; status++ {
			workflowExec(t, "UPDATE brands SET approval=$1,status=$2 WHERE id=$3", approval, status, id)
			cookie := workflowUserCookie(t, u)
			workflowStatus(t, workflowRequest("GET", "/api/v1/me", "", cookie), 200)
		}
	}
}

type captureMailer struct {
	mu       sync.Mutex
	messages []notification.Message
	err      error
}

func (m *captureMailer) Send(_ context.Context, message notification.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	return m.err
}
func resetService() *passwordreset.Service {
	return &passwordreset.Service{Repository: &passwordreset.Repository{DB: testDB, Config: notification.FromEnv()}}
}
func dispatchReset(t *testing.T, user testUser) string {
	t.Helper()
	eventID := workflowID(t, "SELECT id FROM notification_events WHERE user_id=$1 AND state='pending' ORDER BY id DESC LIMIT 1", user.ID)
	// An updatable view restricts worker claims to this fixture's events.
	mailer := &captureMailer{}
	worker := notification.Service{Repository: &notification.Repository{DB: workflowNotificationDB(t, user.ID)}, Config: notification.FromEnv(), Mailer: mailer}
	for i := 0; i < 2000; i++ {
		ok, err := worker.DispatchOne(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		if workflowID(t, "SELECT count(*) FROM notification_events WHERE id=$1 AND state='sent'", eventID) == 1 {
			break
		}
	}
	for _, message := range mailer.messages {
		if message.To == user.Email && strings.Contains(message.Body, "token=") {
			for _, line := range strings.Split(message.Body, "\n") {
				if offset := strings.Index(line, "https://"); offset >= 0 {
					u, err := url.Parse(line[offset:])
					if err != nil {
						t.Fatal(err)
					}
					return u.Query().Get("token")
				}
			}
		}
	}
	t.Fatal("reset mail missing")
	return ""
}
func TestPasswordResetLifecycleAndEnumeration(t *testing.T) {
	u := createTestUser(t, 2)
	cookie := workflowUserCookie(t, u)
	known := workflowRequest("POST", "/api/v1/password-reset/request", fmt.Sprintf(`{"email":%q}`, u.Email))
	unknown := workflowRequest("POST", "/api/v1/password-reset/request", fmt.Sprintf(`{"email":"missing-%d@example.com"}`, time.Now().UnixNano()))
	workflowStatus(t, known, 202)
	workflowStatus(t, unknown, 202)
	if known.Body.String() != unknown.Body.String() {
		t.Fatal("email enumeration response")
	}
	if workflowID(t, "SELECT count(*) FROM password_reset_tokens WHERE user_id=$1 AND expires_at-created_at=INTERVAL '60 minutes'", u.ID) != 1 {
		t.Fatal("wrong reset expiry")
	}
	token := dispatchReset(t, u)
	if len(token) != 43 || strings.Contains(known.Body.String(), token) {
		t.Fatal("token leak")
	}
	var stored []byte
	if err := testDB.QueryRow(context.Background(), "SELECT token_hash FROM password_reset_tokens WHERE user_id=$1", u.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(token))
	if string(stored) != string(h[:]) {
		t.Fatal("token not hashed")
	}
	body := fmt.Sprintf(`{"token":%q,"password":"changed-password"}`, token)
	workflowStatus(t, workflowRequest("POST", "/api/v1/password-reset/confirm", body), 200)
	workflowStatus(t, workflowRequest("POST", "/api/v1/password-reset/confirm", body), 400)
	workflowStatus(t, workflowRequest("GET", "/api/v1/me", "", cookie), 401)
	workflowStatus(t, workflowRequest("POST", "/api/v1/login", fmt.Sprintf(`{"email":%q,"password":"changed-password"}`, u.Email)), 200)
	var hash string
	if err := testDB.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("changed-password")) != nil {
		t.Fatal("new password hash invalid")
	}
}
func TestPasswordResetReissueExpiryAndConcurrentUse(t *testing.T) {
	u := createTestUser(t, 2)
	s := resetService()
	ctx := context.Background()
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	old := dispatchReset(t, u)
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	token := dispatchReset(t, u)
	if !errors.Is(s.Reset(ctx, old, "new-password"), passwordreset.ErrToken) {
		t.Fatal("revoked token accepted")
	}
	workflowExec(t, "UPDATE password_reset_tokens SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=$1 AND revoked_at IS NULL", u.ID)
	if !errors.Is(s.Reset(ctx, token, "new-password"), passwordreset.ErrToken) {
		t.Fatal("expired token accepted")
	}
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	token = dispatchReset(t, u)
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- s.Reset(ctx, token, "new-password") }()
	}
	close(start)
	a, b := <-results, <-results
	if !((a == nil && errors.Is(b, passwordreset.ErrToken)) || (b == nil && errors.Is(a, passwordreset.ErrToken))) {
		t.Fatalf("concurrent reset %v %v", a, b)
	}
}

// Shadow only notification_events with an automatically updatable view. The
// worker executes its real SQL but cannot claim other packages' notifications.
func workflowNotificationDB(t *testing.T, userID int64) *pgxpool.Pool {
	t.Helper()
	schema := fmt.Sprintf("notification_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	workflowExec(t, "CREATE SCHEMA "+quoted)
	t.Cleanup(func() { workflowExec(t, "DROP SCHEMA "+quoted+" CASCADE") })
	workflowExec(t, fmt.Sprintf("CREATE VIEW %s.notification_events AS SELECT * FROM public.notification_events WHERE user_id=%d WITH LOCAL CHECK OPTION", quoted, userID))
	config := testDB.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestNotificationRetriesUnknownAndDeduplication(t *testing.T) {
	u := createTestUser(t, 2)
	ctx := context.Background()
	db := workflowNotificationDB(t, u.ID)
	enqueue := func(key string) int64 {
		t.Helper()
		tx, err := testDB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		for i := 0; i < 2; i++ {
			if err := notification.Enqueue(ctx, tx, key, u.ID, notification.Message{To: u.Email, Subject: "AUMA 運営審査が完了しました。", Body: "審査通知"}, false, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return workflowID(t, "SELECT id FROM notification_events WHERE event_key=$1", key)
	}
	key := fmt.Sprintf("test-notification:%d", u.ID)
	id := enqueue(key)
	mailer := &captureMailer{err: notification.ErrNotAccepted}
	worker := notification.Service{Repository: &notification.Repository{DB: db}, Config: notification.FromEnv(), Mailer: mailer}
	found, err := worker.DispatchOne(ctx)
	if err != nil || !found {
		t.Fatal(err)
	}
	if workflowID(t, "SELECT count(*) FROM notification_events WHERE id=$1 AND state='failed' AND attempts=1 AND last_error='delivery_not_accepted'", id) != 1 {
		t.Fatal("definite failure not retryable")
	}
	if ok, err := worker.DispatchOne(ctx); ok || err != nil {
		t.Fatal("backoff ignored")
	}
	workflowExec(t, "UPDATE notification_events SET next_attempt_at=NOW() WHERE id=$1", id)
	mailer.err = nil
	if ok, err := worker.DispatchOne(ctx); !ok || err != nil {
		t.Fatal(err)
	}
	if ok, err := worker.DispatchOne(ctx); ok || err != nil {
		t.Fatal("sent event resent")
	}
	if len(mailer.messages) != 2 || mailer.messages[0].ID != mailer.messages[1].ID {
		t.Fatal("delivery identity or deduplication broken")
	}
	unknown := enqueue(key + ":unknown")
	mailer.err = errors.New("connection lost with SECRET token")
	if ok, err := worker.DispatchOne(ctx); !ok || err != nil {
		t.Fatal(err)
	}
	if workflowID(t, "SELECT count(*) FROM notification_events WHERE id=$1 AND state='unknown' AND last_error='delivery_unknown'", unknown) != 1 {
		t.Fatal("ambiguous delivery retried or secret saved")
	}
	workflowExec(t, "UPDATE notification_events SET next_attempt_at=NOW()-INTERVAL '1 hour' WHERE id=$1", unknown)
	if ok, err := worker.DispatchOne(ctx); ok || err != nil {
		t.Fatal("unknown event resent")
	}
	interrupted := enqueue(key + ":interrupted")
	workflowExec(t, "UPDATE notification_events SET state='sending',claimed_at=NOW()-INTERVAL '6 minutes' WHERE id=$1", interrupted)
	if ok, err := worker.DispatchOne(ctx); ok || err != nil {
		t.Fatal("interrupted event resent")
	}
	if workflowID(t, "SELECT count(*) FROM notification_events WHERE id=$1 AND state='unknown' AND last_error='worker_interrupted'", interrupted) != 1 {
		t.Fatal("interrupted event not visible")
	}
	_, cookie := workflowAdmin(t)
	r := workflowRequest("GET", "/api/v1/info/notification-events", "", cookie)
	workflowStatus(t, r, 200)
	if strings.Contains(r.Body.String(), "SECRET") || strings.Contains(r.Body.String(), `"body"`) {
		t.Fatal("private notification payload exposed")
	}
}
func TestNotificationConcurrentWorkers(t *testing.T) {
	u := createTestUser(t, 2)
	ctx := context.Background()
	db := workflowNotificationDB(t, u.ID)
	tx, err := testDB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := notification.Enqueue(ctx, tx, fmt.Sprintf("worker-race:%d", u.ID), u.ID, notification.Message{To: u.Email, Subject: "test", Body: "test"}, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{}
	worker := notification.Service{Repository: &notification.Repository{DB: db}, Config: notification.FromEnv(), Mailer: mailer}
	start := make(chan struct{})
	results := make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func() { <-start; _, err := worker.DispatchOne(ctx); results <- err }()
	}
	close(start)
	for i := 0; i < 6; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if len(mailer.messages) != 1 {
		t.Fatalf("duplicate send count %d", len(mailer.messages))
	}
}
func TestResetMailFailureAndReissueCancellation(t *testing.T) {
	u := createTestUser(t, 2)
	s := resetService()
	ctx := context.Background()
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	id := workflowID(t, "SELECT id FROM notification_events WHERE user_id=$1", u.ID)
	var encrypted string
	if err := testDB.QueryRow(ctx, "SELECT body FROM notification_events WHERE id=$1", id).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "https:") || strings.Contains(encrypted, "token=") {
		t.Fatal("plaintext reset link stored")
	}
	mailer := &captureMailer{err: notification.ErrNotAccepted}
	worker := notification.Service{Repository: &notification.Repository{DB: workflowNotificationDB(t, u.ID)}, Mailer: mailer, Config: notification.FromEnv()}
	if _, err := worker.DispatchOne(ctx); err != nil {
		t.Fatal(err)
	}
	if workflowID(t, "SELECT count(*) FROM password_reset_tokens WHERE user_id=$1 AND revoked_at IS NULL", u.ID) != 1 {
		t.Fatal("mail failure revoked token")
	}
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	if workflowID(t, "SELECT count(*) FROM notification_events WHERE id=$1 AND state='cancelled' AND body=''", id) != 1 {
		t.Fatal("superseded reset mail retained")
	}
	if workflowID(t, "SELECT count(*) FROM password_reset_tokens WHERE user_id=$1 AND revoked_at IS NULL", u.ID) != 1 {
		t.Fatal("reissue left multiple live tokens")
	}
	// Expired unsent links are cancelled without invoking the Mailer.
	workflowExec(t, "UPDATE password_reset_tokens SET expires_at=NOW()-INTERVAL '1 second' WHERE user_id=$1", u.ID)
	if ok, err := worker.DispatchOne(ctx); ok || err != nil {
		t.Fatal("expired reset link sent")
	}
	if len(mailer.messages) != 1 {
		t.Fatal("unexpected reset delivery")
	}
}

func workflowIsolatedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	schema := fmt.Sprintf("review_rollback_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	workflowExec(t, "CREATE SCHEMA "+quoted)
	t.Cleanup(func() { workflowExec(t, "DROP SCHEMA "+quoted+" CASCADE") })
	for _, table := range []string{"companies", "users", "user_sessions", "brands", "shops", "admin_users", "review_decisions", "notification_events", "password_reset_tokens"} {
		workflowExec(t, "CREATE TABLE "+quoted+"."+pgx.Identifier{table}.Sanitize()+" (LIKE public."+pgx.Identifier{table}.Sanitize()+" INCLUDING ALL)")
	}
	config := testDB.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}
func TestReviewDatabaseErrorsRollback(t *testing.T) {
	adminID, _ := workflowAdmin(t)
	ctx := context.Background()
	for _, kind := range []string{"brands", "shops"} {
		for _, deferred := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_deferred_%t", kind, deferred), func(t *testing.T) {
				typ := int16(2)
				if kind == "shops" {
					typ = 1
				}
				u := createTestUser(t, typ)
				var id int64
				if kind == "brands" {
					id = workflowID(t, "SELECT id FROM brands WHERE company_id=$1", u.CompanyID)
				} else {
					id = workflowID(t, "INSERT INTO shops(company_id,name,registration_plan) VALUES ($1,'rollback shop',10) RETURNING id", u.CompanyID)
				}
				workflowExec(t, "UPDATE "+kind+" SET submitted_at=NOW(),status=1 WHERE id=$1", id)
				db := workflowIsolatedDB(t)
				for _, entry := range []struct {
					table, where string
					id           int64
				}{{"companies", "id", u.CompanyID}, {"users", "id", u.ID}, {kind, "id", id}, {"admin_users", "id", adminID}} {
					if _, err := db.Exec(ctx, fmt.Sprintf("INSERT INTO %s SELECT * FROM public.%s WHERE %s=$1", entry.table, entry.table, entry.where), entry.id); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.Exec(ctx, `CREATE FUNCTION fail_review() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private-database-error'; END $$`); err != nil {
					t.Fatal(err)
				}
				trigger := "CREATE TRIGGER fail_review AFTER INSERT ON notification_events FOR EACH ROW EXECUTE FUNCTION fail_review()"
				if deferred {
					trigger = "CREATE CONSTRAINT TRIGGER fail_review AFTER INSERT ON notification_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_review()"
				}
				if _, err := db.Exec(ctx, trigger); err != nil {
					t.Fatal(err)
				}
				var before, after string
				if err := db.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM "+kind+" e WHERE id=$1", id).Scan(&before); err != nil {
					t.Fatal(err)
				}
				service := review.Service{Repository: &review.Repository{DB: db}}
				if err := service.Decide(ctx, kind, id, u.CompanyID, adminID, 1); err == nil {
					t.Fatal("injected failure accepted")
				}
				if err := db.QueryRow(ctx, "SELECT to_jsonb(e)::text FROM "+kind+" e WHERE id=$1", id).Scan(&after); err != nil {
					t.Fatal(err)
				}
				if before != after {
					t.Fatal("review failure changed target")
				}
				var count int
				if err := db.QueryRow(ctx, "SELECT (SELECT count(*) FROM review_decisions)+(SELECT count(*) FROM notification_events)").Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("review left partial event/audit")
				}
			})
		}
	}
}
func TestPasswordResetDatabaseRollback(t *testing.T) {
	ctx := context.Background()
	u := createTestUser(t, 2)
	s := resetService()
	if err := s.Request(ctx, u.Email); err != nil {
		t.Fatal(err)
	}
	token := dispatchReset(t, u)
	for _, deferred := range []bool{false, true} {
		t.Run(fmt.Sprint(deferred), func(t *testing.T) {
			db := workflowIsolatedDB(t)
			for _, entry := range []struct{ table, where string }{{"users", "id"}, {"password_reset_tokens", "user_id"}, {"notification_events", "user_id"}} {
				if _, err := db.Exec(ctx, fmt.Sprintf("INSERT INTO %s SELECT * FROM public.%s WHERE %s=$1", entry.table, entry.table, entry.where), u.ID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(ctx, `CREATE FUNCTION fail_password() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'private-password-error'; END $$`); err != nil {
				t.Fatal(err)
			}
			trigger := "CREATE TRIGGER fail_password AFTER UPDATE ON users FOR EACH ROW EXECUTE FUNCTION fail_password()"
			if deferred {
				trigger = "CREATE CONSTRAINT TRIGGER fail_password AFTER UPDATE ON users DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_password()"
			}
			if _, err := db.Exec(ctx, trigger); err != nil {
				t.Fatal(err)
			}
			service := passwordreset.Service{Repository: &passwordreset.Repository{DB: db, Config: notification.FromEnv()}}
			if err := service.Reset(ctx, token, "rollback-password"); err == nil {
				t.Fatal("injected password error accepted")
			}
			var count int
			var hash string
			if err := db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&hash); err != nil {
				t.Fatal(err)
			}
			if bcrypt.CompareHashAndPassword([]byte(hash), []byte(testPassword)) != nil {
				t.Fatal("failed reset changed password")
			}
			if err := db.QueryRow(ctx, "SELECT count(*) FROM password_reset_tokens WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL", u.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatal("failed reset consumed token")
			}
		})
	}
}
func TestInfoLoginRejectsGeneralCredentialsAndRateLimits(t *testing.T) {
	u := createTestUser(t, 2)
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, u.Email, testPassword)
	for i := 0; i < 5; i++ {
		workflowStatus(t, workflowRequest("POST", "/api/v1/info/login", body), 401)
	}
	workflowStatus(t, workflowRequest("POST", "/api/v1/info/login", body), 429)
}

func TestReviewDeliveryFailurePreservesDecisionAndPassword(t *testing.T) {
	adminID, _ := workflowAdmin(t)
	u := createTestUser(t, 2)
	id := workflowID(t, "SELECT id FROM brands WHERE company_id=$1", u.CompanyID)
	workflowExec(t, "UPDATE brands SET submitted_at=NOW(),status=1 WHERE id=$1", id)
	service := review.Service{Repository: &review.Repository{DB: testDB}}
	if err := service.Decide(context.Background(), "brands", id, u.CompanyID, adminID, 1); err != nil {
		t.Fatal(err)
	}
	worker := notification.Service{Repository: &notification.Repository{DB: workflowNotificationDB(t, u.ID)}, Config: notification.FromEnv(), Mailer: &captureMailer{err: notification.ErrNotAccepted}}
	if _, err := worker.DispatchOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workflowID(t, "SELECT count(*) FROM brands WHERE id=$1 AND approval=1 AND status=1 AND submitted_at IS NOT NULL", id) != 1 {
		t.Fatal("delivery failure rolled back review")
	}
	cookie := workflowUserCookie(t, u)
	workflowStatus(t, workflowRequest("GET", "/api/v1/me", "", cookie), 200)
}
func TestPasswordResetInvalidInputPreservesState(t *testing.T) {
	u := createTestUser(t, 2)
	s := resetService()
	if err := s.Request(context.Background(), u.Email); err != nil {
		t.Fatal(err)
	}
	token := dispatchReset(t, u)
	for _, input := range []struct{ token, password string }{{token, "short"}, {token, strings.Repeat("a", 73)}, {"bad", "new-password"}, {strings.Repeat("B", 43), "new-password"}} {
		if err := s.Reset(context.Background(), input.token, input.password); err == nil {
			t.Fatal("invalid reset input accepted")
		}
	}
	if workflowID(t, "SELECT count(*) FROM password_reset_tokens WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL", u.ID) != 1 {
		t.Fatal("invalid request consumed reset token")
	}
	var hash string
	if err := testDB.QueryRow(context.Background(), "SELECT password_hash FROM users WHERE id=$1", u.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(testPassword)) != nil {
		t.Fatal("invalid reset changed password")
	}
}
