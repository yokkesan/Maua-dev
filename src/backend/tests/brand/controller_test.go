package brand_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

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

	var response struct {
		Brand struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
			Phonetic  string `json:"phonetic"`
			Tel       string `json:"tel"`
			Email     string `json:"email"`
		} `json:"brand"`
	}

	if err := json.Unmarshal(
		recorder.Body.Bytes(),
		&response,
	); err != nil {
		t.Fatalf(
			"failed to decode response body: %v",
			err,
		)
	}

	if response.Brand.CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id %d, got %d",
			user.CompanyID,
			response.Brand.CompanyID,
		)
	}

	if response.Brand.Name != "Brand APIテストブランド" {
		t.Fatalf(
			"expected brand name %q, got %q",
			"Brand APIテストブランド",
			response.Brand.Name,
		)
	}
}
