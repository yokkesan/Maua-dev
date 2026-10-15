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
		Brands []struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
			Phonetic  string `json:"phonetic"`
			Tel       string `json:"tel"`
			Email     string `json:"email"`
		} `json:"brands"`
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

	if len(response.Brands) != 1 {
		t.Fatalf(
			"expected 1 brand, got %d: %s",
			len(response.Brands),
			recorder.Body.String(),
		)
	}

	if response.Brands[0].CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id %d, got %d",
			user.CompanyID,
			response.Brands[0].CompanyID,
		)
	}

	if response.Brands[0].Name != "Brand APIテストブランド" {
		t.Fatalf(
			"expected brand name %q, got %q",
			"Brand APIテストブランド",
			response.Brands[0].Name,
		)
	}
}

func TestBrandsReturnsMultipleBrandsForSameCompany(t *testing.T) {
	user := createTestUser(t, 2)

	// 同じ会社に2件目のブランドを登録
	_, err := testDB.Exec(
		t.Context(),
		`
			INSERT INTO brands (
				company_id,
				name,
				phonetic,
				tel,
				email
			)
			VALUES ($1, $2, $3, $4, $5)
		`,
		user.CompanyID,
		"追加テストブランド",
		"ツイカテストブランド",
		"03-9999-9999",
		"additional-brand@example.com",
	)
	if err != nil {
		t.Fatalf("failed to create second brand: %v", err)
	}

	// ログイン
	loginBody, err := json.Marshal(map[string]string{
		"email":    user.Email,
		"password": testPassword,
	})
	if err != nil {
		t.Fatalf("failed to encode login request: %v", err)
	}

	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(loginBody),
	)
	loginReq.Header.Set("Content-Type", "application/json")

	loginRecorder := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(loginRecorder, loginReq)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf(
			"login failed: %d: %s",
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

	// ブランド一覧取得
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/brands",
		nil,
	)
	req.AddCookie(sessionCookie)

	recorder := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d: %s",
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var response struct {
		Brands []struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
		} `json:"brands"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Brands) != 2 {
		t.Fatalf(
			"expected 2 brands, got %d: %s",
			len(response.Brands),
			recorder.Body.String(),
		)
	}

	for _, b := range response.Brands {
		if b.CompanyID != user.CompanyID {
			t.Fatalf(
				"another company brand returned: company_id=%d",
				b.CompanyID,
			)
		}
	}
}
