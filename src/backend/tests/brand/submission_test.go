package brand_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	submissioncontroller "backend/controllers/registrationsubmission"
	"backend/internal/auth"
	"backend/internal/registrationsubmission"
	beego "github.com/beego/beego/v2/server/web"
	beegoctx "github.com/beego/beego/v2/server/web/context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type submissionSnapshot struct {
	State     registrationsubmission.State
	UpdatedAt time.Time
}

func submissionTarget(t *testing.T) (testUser, int64, *http.Cookie) {
	t.Helper()
	user := createTestUser(t, 2)
	var id int64
	if err := testDB.QueryRow(context.Background(), "SELECT id FROM brands WHERE company_id=$1 ORDER BY id LIMIT 1", user.CompanyID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(context.Background(), "UPDATE brands SET updated_at='2000-01-01T00:00:00Z' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	seedSubmissionRequirements(t, testDB, id, user.CompanyID)
	return user, id, loginForBrandCreateTest(t, user.Email)
}

func submissionRequest(method, id string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/brands/"+id+"/submission", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	return rec
}

func readSubmissionSnapshot(t *testing.T, db *pgxpool.Pool, id int64) submissionSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var result submissionSnapshot
	if err := db.QueryRow(ctx, "SELECT id, submitted_at, approval, status, updated_at FROM brands WHERE id=$1", id).Scan(
		&result.State.ID, &result.State.SubmittedAt, &result.State.Approval, &result.State.Status, &result.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}
	return result
}

func requireSubmissionHTTP(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("want HTTP %d, got %d: %s", want, rec.Code, rec.Body.String())
	}
}

func TestSubmissionLifecycle(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	target := fmt.Sprint(id)
	before := readSubmissionSnapshot(t, testDB, id)
	rec := submissionRequest(http.MethodGet, target, cookie)
	requireSubmissionHTTP(t, rec, http.StatusOK)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 4 || string(raw["submitted_at"]) != "null" {
		t.Fatalf("unexpected initial response: %s", rec.Body.String())
	}
	var initial registrationsubmission.State
	if err := json.Unmarshal(rec.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(initial, before.State) {
		t.Fatalf("unexpected initial state: %+v", initial)
	}
	if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
		t.Fatal("GET changed DB state")
	}

	rec = submissionRequest(http.MethodPost, target, cookie)
	requireSubmissionHTTP(t, rec, http.StatusOK)
	var submitted registrationsubmission.State
	if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	saved := readSubmissionSnapshot(t, testDB, id)
	if saved.State.ID != id || saved.State.SubmittedAt == nil || saved.State.Approval != 0 || saved.State.Status != 0 {
		t.Fatalf("unexpected saved state: %+v", saved)
	}
	if !saved.UpdatedAt.After(before.UpdatedAt) || !saved.UpdatedAt.Equal(*saved.State.SubmittedAt) {
		t.Fatal("submitted_at and updated_at were not saved together")
	}
	if submitted.SubmittedAt == nil || !submitted.SubmittedAt.Equal(*saved.State.SubmittedAt) || submitted.ID != id || submitted.Approval != 0 || submitted.Status != 0 {
		t.Fatal("POST response differs from DB")
	}

	rec = submissionRequest(http.MethodGet, target, cookie)
	requireSubmissionHTTP(t, rec, http.StatusOK)
	var fetched registrationsubmission.State
	if err := json.Unmarshal(rec.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.SubmittedAt == nil || !fetched.SubmittedAt.Equal(*saved.State.SubmittedAt) || fetched.ID != id || fetched.Approval != 0 || fetched.Status != 0 {
		t.Fatal("GET response differs from DB")
	}
	requireSubmissionHTTP(t, submissionRequest(http.MethodPost, target, cookie), http.StatusConflict)
	if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(saved, after) {
		t.Fatal("duplicate submission changed DB state")
	}
}

func TestSubmissionRejectsStates(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	submitted := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name             string
		approval, status int16
		submitted        *time.Time
	}{
		{"approved", 1, 0, nil},
		{"rejected", 2, 0, nil},
		{"published", 0, 1, nil},
		{"approved_published", 1, 1, nil},
		{"rejected_published", 2, 1, nil},
		{"already_submitted", 0, 0, &submitted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := testDB.Exec(context.Background(), "UPDATE brands SET approval=$1, status=$2, submitted_at=$3 WHERE id=$4", tc.approval, tc.status, tc.submitted, id); err != nil {
				t.Fatal(err)
			}
			before := readSubmissionSnapshot(t, testDB, id)
			requireSubmissionHTTP(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), http.StatusConflict)
			if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
				t.Fatal("rejected submission changed DB state")
			}
		})
	}
}

func TestSubmissionAccessAndIDs(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	_, otherID, _ := submissionTarget(t)
	wrongUser := createTestUser(t, 1)
	wrongCookie := loginForBrandCreateTest(t, wrongUser.Email)
	before := readSubmissionSnapshot(t, testDB, id)
	otherBefore := readSubmissionSnapshot(t, testDB, otherID)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			requireSubmissionHTTP(t, submissionRequest(method, fmt.Sprint(id), nil), http.StatusUnauthorized)
			requireSubmissionHTTP(t, submissionRequest(method, fmt.Sprint(id), wrongCookie), http.StatusForbidden)
			requireSubmissionHTTP(t, submissionRequest(method, fmt.Sprint(otherID), cookie), http.StatusNotFound)
			for _, target := range []string{"0", "-1", "abc", "9223372036854775808"} {
				requireSubmissionHTTP(t, submissionRequest(method, target, cookie), http.StatusBadRequest)
			}
			requireSubmissionHTTP(t, submissionRequest(method, "9223372036854775807", cookie), http.StatusNotFound)
		})
	}
	if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid access changed own target")
	}
	if after := readSubmissionSnapshot(t, testDB, otherID); !reflect.DeepEqual(otherBefore, after) {
		t.Fatal("invalid access changed another company's target")
	}
}

func TestSubmissionConcurrent(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	const count = 8
	start := make(chan struct{})
	results := make(chan int, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- submissionRequest(http.MethodPost, fmt.Sprint(id), cookie).Code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for code := range results {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected concurrent response: %d", code)
		}
	}
	if successes != 1 || conflicts != count-1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	saved := readSubmissionSnapshot(t, testDB, id)
	if saved.State.SubmittedAt == nil || saved.State.Approval != 0 || saved.State.Status != 0 {
		t.Fatalf("unexpected concurrent state: %+v", saved)
	}
}

// Error injection is confined to a disposable schema in auma_test.
// Public tables, triggers, and other tests' rows are never changed.
func TestSubmissionDatabaseRollback(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(fmt.Sprintf("deferred_%t", deferred), func(t *testing.T) {
			ctx := context.Background()
			var database string
			if err := testDB.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
				t.Fatal(err)
			}
			if database != "auma_test" {
				t.Fatal("rollback test requires auma_test")
			}
			schema := fmt.Sprintf("submission_brand_%d", time.Now().UnixNano())
			quoted := pgx.Identifier{schema}.Sanitize()
			if _, err := testDB.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := testDB.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
					t.Errorf("cleanup schema: %v", err)
				}
			})
			config := testDB.Config()
			config.ConnConfig.RuntimeParams["search_path"] = schema
			isolated, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(isolated.Close)
			for _, table := range []string{"companies", "brands", "brand_images", "brand_categories", "categories", "first_categories", "second_categories", "third_categories", "brand_price_screenings", "brand_price_screening_products"} {
				if _, err := isolated.Exec(ctx, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" (LIKE public."+pgx.Identifier{table}.Sanitize()+" INCLUDING DEFAULTS)"); err != nil {
					t.Fatal(err)
				}
			}
			var companyID, targetID int64
			if err := isolated.QueryRow(ctx, "INSERT INTO companies (company_type) VALUES (0) RETURNING id").Scan(&companyID); err != nil {
				t.Fatal(err)
			}
			if err := isolated.QueryRow(ctx, "INSERT INTO brands (company_id, name) VALUES ($1, 'test') RETURNING id", companyID).Scan(&targetID); err != nil {
				t.Fatal(err)
			}
			seedSubmissionRequirements(t, isolated, targetID, companyID)
			if _, err := isolated.Exec(ctx, `CREATE FUNCTION fail_submission() RETURNS trigger LANGUAGE plpgsql AS $$
            BEGIN RAISE EXCEPTION 'submission-test-internal-detail'; END $$;`); err != nil {
				t.Fatal(err)
			}
			trigger := "CREATE TRIGGER fail_submission AFTER UPDATE ON brands FOR EACH ROW EXECUTE FUNCTION fail_submission()"
			if deferred {
				trigger = "CREATE CONSTRAINT TRIGGER fail_submission AFTER UPDATE ON brands DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION fail_submission()"
			}
			if _, err := isolated.Exec(ctx, trigger); err != nil {
				t.Fatal(err)
			}
			before := readSubmissionSnapshot(t, isolated, targetID)
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/brands/%d/submission", targetID), nil)
			req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 1, CompanyID: companyID, UserType: 2}))
			rec := httptest.NewRecorder()
			controllerContext := beegoctx.NewContext()
			controllerContext.Reset(rec, req)
			controllerContext.Input.SetParam(":id", fmt.Sprint(targetID))
			controller := &submissioncontroller.Controller{
				Service: registrationsubmission.NewService(registrationsubmission.NewPostgresRepository(isolated)),
				Entity:  registrationsubmission.Brand,
			}
			controller.Init(controllerContext, "Controller", "Post", nil)
			controller.Post()
			requireSubmissionHTTP(t, rec, http.StatusInternalServerError)
			if strings.Contains(rec.Body.String(), "submission-test-internal-detail") || !strings.Contains(rec.Body.String(), "internal server error") {
				t.Fatalf("unsafe error response: %s", rec.Body.String())
			}
			if after := readSubmissionSnapshot(t, isolated, targetID); !reflect.DeepEqual(before, after) {
				t.Fatal("failed transaction changed DB state")
			}
		})
	}
}

// Only dedicated rows are created; no production files or pre-existing masters are used.
func seedSubmissionRequirements(t *testing.T, db *pgxpool.Pool, id, companyID int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.Exec(ctx, `UPDATE companies SET name='申請会社', phonetic='シンセイカイシャ',
        representative_name='担当代表', representative_phonetic='タントウダイヒョウ',
        post_code='100-0001', address='東京都', tel='0312345678' WHERE id=$1`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE brands SET name='申請対象', phonetic='シンセイタイショウ',
        concept='申請用コンセプト', target_min='0', target_max='60', web_url='形式は検証しない',
        net_shop=0, contact_person_name='担当者', contact_person_phonetic='タントウシャ',
        contact_person_tel='内線123', updated_at='2000-01-01T00:00:00Z' , price_min='100', price_max='200', base='拠点' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var first, second, category int64
	if err := db.QueryRow(ctx, `INSERT INTO first_categories(name) VALUES ('submission fixture') RETURNING id`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, statement := range []string{
			"DELETE FROM brand_price_screenings WHERE first_category_id=$1",
			"DELETE FROM brand_categories WHERE category_id IN (SELECT id FROM categories WHERE first_id=$1)",
			"DELETE FROM categories WHERE first_id=$1",
			"DELETE FROM third_categories WHERE second_category_id IN (SELECT id FROM second_categories WHERE first_category_id=$1)",
			"DELETE FROM second_categories WHERE first_category_id=$1",
			"DELETE FROM first_categories WHERE id=$1",
		} {
			if _, err := db.Exec(cleanupCtx, statement, first); err != nil {
				t.Errorf("cleanup submission fixture: %v", err)
			}
		}
	})
	if _, err := db.Exec(ctx, "INSERT INTO brand_price_screenings(brand_id,first_category_id,saved_at) VALUES ($1,$2,NOW())", id, first); err != nil {
		t.Fatal(err)
	}

	if err := db.QueryRow(ctx, `INSERT INTO second_categories(first_category_id,name) VALUES ($1,'submission second') RETURNING id`, first).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO categories(first_id,second_id) VALUES ($1,$2) RETURNING id`, first, second).Scan(&category); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)`, id, category); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO brand_images(brand_id,slot_number,storage_key) VALUES ($1,1,'submission-fixture/1.png'),($1,2,'submission-fixture/2.png')`, id); err != nil {
		t.Fatal(err)
	}
}

func requireSubmissionFields(t *testing.T, rec *httptest.ResponseRecorder, fields ...string) {
	t.Helper()
	requireSubmissionHTTP(t, rec, http.StatusUnprocessableEntity)
	var response struct {
		Error  string   `json:"error"`
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "required fields are missing" || !reflect.DeepEqual(response.Fields, fields) {
		t.Fatalf("unexpected validation response: %s", rec.Body.String())
	}
}

func TestSubmissionRequiredFields(t *testing.T) {
	user, id, cookie := submissionTarget(t)
	cases := []struct {
		table, column, field string
		value                any
	}{
		{"companies", "name", "company.name", ""},
		{"companies", "name", "company.name", " \t\u3000"},
		{"companies", "phonetic", "company.phonetic", ""},
		{"companies", "representative_name", "company.representative_name", ""},
		{"companies", "representative_name", "company.representative_name", " \t\u3000"},
		{"companies", "representative_phonetic", "company.representative_phonetic", ""},
		{"companies", "post_code", "company.post_code", ""},
		{"companies", "post_code", "company.post_code", "1000001"},
		{"companies", "address", "company.address", ""},
		{"companies", "tel", "company.tel", ""},
		{"companies", "tel", "company.tel", "03-1234-5678"},
		{"companies", "tel", "company.tel", "1312345678"},
		{"brands", "name", "name", ""},
		{"brands", "phonetic", "phonetic", ""},
		{"brands", "concept", "concept", ""},
		{"brands", "concept", "concept", strings.Repeat("あ", 251)},
		{"brands", "concept", "concept", strings.Repeat("😀", 126)},
		{"brands", "target_min", "target_min", nil},
		{"brands", "target_max", "target_max", ""},
		{"brands", "web_url", "web_url", ""},
		{"brands", "net_shop", "net_shop", nil},
		{"brands", "contact_person_name", "contact_person_name", ""},
		{"brands", "contact_person_phonetic", "contact_person_phonetic", ""},
		{"brands", "contact_person_tel", "contact_person_tel", ""},
		{"brands", "price_min", "price_min", ""},
		{"brands", "price_max", "price_max", ""},
		{"brands", "base", "base", ""},
	}
	// Nullable columns must also reject stored SQL NULL, apart from the explicitly
	// null-only target_min/net_shop cases already included above.
	baseCases := append([]struct {
		table, column, field string
		value                any
	}{}, cases...)
	seen := make(map[string]bool)
	for _, tc := range baseCases {
		key := tc.table + "." + tc.column
		if seen[key] || tc.column == "net_shop" || tc.column == "target_min" || (tc.table == "brands" && tc.column == "name") {
			continue
		}
		seen[key] = true
		tc.value = nil
		cases = append(cases, tc)
	}
	for index, tc := range cases {
		t.Run(fmt.Sprintf("%s_%d", tc.field, index), func(t *testing.T) {
			targetID := id
			if tc.table == "companies" {
				targetID = user.CompanyID
			}
			var original any
			query := "SELECT " + tc.column + " FROM " + tc.table + " WHERE id=$1"
			if err := testDB.QueryRow(context.Background(), query, targetID).Scan(&original); err != nil {
				t.Fatal(err)
			}
			update := "UPDATE " + tc.table + " SET " + tc.column + "=$1 WHERE id=$2"
			if _, err := testDB.Exec(context.Background(), update, tc.value, targetID); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := testDB.Exec(context.Background(), update, original, targetID); err != nil {
					t.Error(err)
				}
			})
			before := readSubmissionSnapshot(t, testDB, id)
			// GET remains available even when required details are incomplete.
			requireSubmissionHTTP(t, submissionRequest(http.MethodGet, fmt.Sprint(id), cookie), http.StatusOK)
			requireSubmissionFields(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), tc.field)
			if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
				t.Fatal("validation failure changed submission timestamps/state")
			}
		})
	}
}

func TestSubmissionImagesAndCategories(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	ctx := context.Background()
	before := readSubmissionSnapshot(t, testDB, id)
	if _, err := testDB.Exec(ctx, "DELETE FROM brand_images WHERE brand_id=$1 AND slot_number=2", id); err != nil {
		t.Fatal(err)
	}
	requireSubmissionFields(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), "main_images")
	if _, err := testDB.Exec(ctx, "DELETE FROM brand_images WHERE brand_id=$1", id); err != nil {
		t.Fatal(err)
	}
	requireSubmissionFields(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), "main_images")
	// Two rows do not suffice when the second main slot has no stored image key.
	if _, err := testDB.Exec(ctx, `INSERT INTO brand_images(brand_id,slot_number,storage_key) VALUES ($1,1,'fixture/1.png'),($1,2,'')`, id); err != nil {
		t.Fatal(err)
	}
	requireSubmissionFields(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), "main_images")
	if _, err := testDB.Exec(ctx, `UPDATE brand_images SET storage_key='fixture/2.png' WHERE brand_id=$1 AND slot_number=2`, id); err != nil {
		t.Fatal(err)
	}
	var categoryID int64
	if err := testDB.QueryRow(ctx, "SELECT category_id FROM brand_categories WHERE brand_id=$1", id).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(ctx, "DELETE FROM brand_categories WHERE brand_id=$1", id); err != nil {
		t.Fatal(err)
	}
	// A first-category master alone cannot represent a valid second/third selection.
	requireSubmissionFields(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), "category")
	if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid images/category changed state")
	}
	if _, err := testDB.Exec(ctx, "INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)", id, categoryID); err != nil {
		t.Fatal(err)
	}
	var secondID, thirdID int64
	if err := testDB.QueryRow(ctx, "SELECT second_id FROM categories WHERE id=$1", categoryID).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	if err := testDB.QueryRow(ctx, "INSERT INTO third_categories(second_category_id,name) VALUES ($1,'submission third') RETURNING id", secondID).Scan(&thirdID); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(ctx, "UPDATE categories SET third_id=$1 WHERE id=$2", thirdID, categoryID); err != nil {
		t.Fatal(err)
	}
	requireSubmissionHTTP(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), http.StatusOK)
}

func TestSubmissionLegacyPresenceChecks(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	// Legacy Vue checks target_min and net_shop for null, not truthiness.
	// It does not validate URL format or use the company-tel regex for contacts.
	if _, err := testDB.Exec(context.Background(), `UPDATE brands SET target_min='', net_shop=0,
        concept=$1, web_url='not a URL', contact_person_tel='内線のみ' WHERE id=$2`, strings.Repeat("😀", 125), id); err != nil {
		t.Fatal(err)
	}
	requireSubmissionHTTP(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), http.StatusOK)
}

func TestSubmissionConflictBeforeRequiredFields(t *testing.T) {
	_, id, cookie := submissionTarget(t)
	if _, err := testDB.Exec(context.Background(), "UPDATE brands SET approval=1, concept=NULL WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	before := readSubmissionSnapshot(t, testDB, id)
	requireSubmissionHTTP(t, submissionRequest(http.MethodPost, fmt.Sprint(id), cookie), http.StatusConflict)
	if after := readSubmissionSnapshot(t, testDB, id); !reflect.DeepEqual(before, after) {
		t.Fatal("conflict changed state")
	}
}
