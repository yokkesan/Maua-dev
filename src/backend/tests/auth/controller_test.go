package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

func TestLoginSuccess(t *testing.T) {
	user := createTestUser(t, 2)

	body, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": testPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode request body: %v",
			err,
		)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(body),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	response := recorder.Result()

	var sessionCookie *http.Cookie

	for _, cookie := range response.Cookies() {
		if cookie.Name == "auma_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("auma_session cookie should be set")
	}

	if sessionCookie.Value == "" {
		t.Fatal("auma_session cookie should not be empty")
	}

	if !sessionCookie.HttpOnly {
		t.Fatal("auma_session cookie should be HttpOnly")
	}
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	user := createTestUser(t, 2)

	body, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": "WrongPassword123!",
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode request body: %v",
			err,
		)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(body),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusUnauthorized,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	user := createTestUser(t, 2)

	loginBody, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": testPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode login request body: %v",
			err,
		)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(loginBody),
	)

	loginReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	loginRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		loginRecorder,
		loginReq,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected login status %d, got %d: %s",
			http.StatusOK,
			loginRecorder.Code,
			loginRecorder.Body.String(),
		)
	}

	var sessionCookie *http.Cookie

	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == "auma_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("auma_session cookie should be set")
	}

	logoutReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/logout",
		nil,
	)

	logoutReq.AddCookie(sessionCookie)

	logoutRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		logoutRecorder,
		logoutReq,
	)

	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected logout status %d, got %d: %s",
			http.StatusOK,
			logoutRecorder.Code,
			logoutRecorder.Body.String(),
		)
	}

	meReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/me",
		nil,
	)

	meReq.AddCookie(sessionCookie)

	meRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		meRecorder,
		meReq,
	)

	if meRecorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d after logout, got %d: %s",
			http.StatusUnauthorized,
			meRecorder.Code,
			meRecorder.Body.String(),
		)
	}
}

func TestBrandsRejectsUnauthenticated(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/brands",
		nil,
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusUnauthorized,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestBrandsRejectsShopUser(t *testing.T) {
	user := createTestUser(t, 1)

	loginBody, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": testPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode login request body: %v",
			err,
		)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(loginBody),
	)

	loginReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	loginRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		loginRecorder,
		loginReq,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected login status %d, got %d: %s",
			http.StatusOK,
			loginRecorder.Code,
			loginRecorder.Body.String(),
		)
	}

	var sessionCookie *http.Cookie

	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == "auma_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("auma_session cookie should be set")
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/brands",
		nil,
	)

	req.AddCookie(sessionCookie)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusForbidden,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestBrandsReturnsBrandUserCompanyBrand(t *testing.T) {
	user := createTestUser(t, 2)

	loginBody, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": testPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode login request body: %v",
			err,
		)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(loginBody),
	)

	loginReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	loginRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		loginRecorder,
		loginReq,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected login status %d, got %d: %s",
			http.StatusOK,
			loginRecorder.Code,
			loginRecorder.Body.String(),
		)
	}

	var sessionCookie *http.Cookie

	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == "auma_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("auma_session cookie should be set")
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/brands",
		nil,
	)

	req.AddCookie(sessionCookie)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if !bytes.Contains(
		recorder.Body.Bytes(),
		[]byte(`"company_id":`),
	) {
		t.Fatalf(
			"response does not contain company_id: %s",
			recorder.Body.String(),
		)
	}
}

func TestMeRejectsUnauthenticated(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/me",
		nil,
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusUnauthorized,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestMeReturnsAuthenticatedUser(t *testing.T) {
	user := createTestUser(t, 2)

	loginBody, err := json.Marshal(
		map[string]string{
			"email":    user.Email,
			"password": testPassword,
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode login request body: %v",
			err,
		)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(loginBody),
	)

	loginReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	loginRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		loginRecorder,
		loginReq,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected login status %d, got %d: %s",
			http.StatusOK,
			loginRecorder.Code,
			loginRecorder.Body.String(),
		)
	}

	var sessionCookie *http.Cookie

	for _, cookie := range loginRecorder.Result().Cookies() {
		if cookie.Name == "auma_session" {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("auma_session cookie should be set")
	}

	meReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/me",
		nil,
	)

	meReq.AddCookie(sessionCookie)

	meRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		meRecorder,
		meReq,
	)

	if meRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusOK,
			meRecorder.Code,
			meRecorder.Body.String(),
		)
	}

	if !bytes.Contains(
		meRecorder.Body.Bytes(),
		[]byte(user.Email),
	) {
		t.Fatalf(
			"response does not contain expected email: %s",
			meRecorder.Body.String(),
		)
	}
}
