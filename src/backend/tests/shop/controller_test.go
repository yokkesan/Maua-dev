package shop_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

func TestShopsRejectsUnauthenticated(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/shops",
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

func TestShopsRejectsBrandUser(t *testing.T) {
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
		"/api/v1/shops",
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

func TestShopsReturnsShopUserCompanyShop(t *testing.T) {
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
		"/api/v1/shops",
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
		Shop struct {
			ID               int64  `json:"id"`
			CompanyID        int64  `json:"company_id"`
			Name             string `json:"name"`
			Phonetic         string `json:"phonetic"`
			Tel              string `json:"tel"`
			Email            string `json:"email"`
			RegistrationPlan int16  `json:"registration_plan"`
		} `json:"shop"`
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

	if response.Shop.CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id %d, got %d",
			user.CompanyID,
			response.Shop.CompanyID,
		)
	}

	if response.Shop.Name != "Shop APIテスト店舗" {
		t.Fatalf(
			"expected shop name %q, got %q",
			"Shop APIテスト店舗",
			response.Shop.Name,
		)
	}
}

func TestShopsDoesNotReturnAnotherCompanyShop(t *testing.T) {
	user := createTestUser(t, 1)

	var otherCompanyID int64

	err := testDB.QueryRow(
		t.Context(),
		`
			INSERT INTO companies (
				name,
				company_type
			)
			VALUES ($1, $2)
			RETURNING id
		`,
		"他社Shopテスト株式会社",
		int16(0),
	).Scan(&otherCompanyID)
	if err != nil {
		t.Fatalf(
			"failed to create other company: %v",
			err,
		)
	}

	t.Cleanup(func() {
		_, _ = testDB.Exec(
			t.Context(),
			`
				DELETE FROM companies
				WHERE id = $1
			`,
			otherCompanyID,
		)
	})

	otherEmail := fmt.Sprintf(
		"other-shop-%d@example.com",
		otherCompanyID,
	)

	_, err = testDB.Exec(
		t.Context(),
		`
			INSERT INTO shops (
				company_id,
				name,
				phonetic,
				tel,
				email,
				registration_plan
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6
			)
		`,
		otherCompanyID,
		"他社Shop",
		"タシャショップ",
		"097-111-1111",
		otherEmail,
		int16(10),
	)
	if err != nil {
		t.Fatalf(
			"failed to create other shop: %v",
			err,
		)
	}

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
		fmt.Sprintf(
			"/api/v1/shops?company_id=%d",
			otherCompanyID,
		),
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
		Shop struct {
			CompanyID int64 `json:"company_id"`
		} `json:"shop"`
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

	if response.Shop.CompanyID != user.CompanyID {
		t.Fatalf(
			"expected own company_id %d, got %d",
			user.CompanyID,
			response.Shop.CompanyID,
		)
	}

	if response.Shop.CompanyID == otherCompanyID {
		t.Fatalf(
			"another company shop must not be returned: %d",
			otherCompanyID,
		)
	}
}
