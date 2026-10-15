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

	// 複数ショップ対応のレスポンス
	var response struct {
		Shops []struct {
			ID               int64  `json:"id"`
			CompanyID        int64  `json:"company_id"`
			Name             string `json:"name"`
			Phonetic         string `json:"phonetic"`
			Tel              string `json:"tel"`
			Email            string `json:"email"`
			RegistrationPlan int16  `json:"registration_plan"`
		} `json:"shops"`
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

	// このテストでは自社ショップを1件作成している
	if len(response.Shops) != 1 {
		t.Fatalf(
			"expected 1 shop, got %d: %s",
			len(response.Shops),
			recorder.Body.String(),
		)
	}

	if response.Shops[0].CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id %d, got %d",
			user.CompanyID,
			response.Shops[0].CompanyID,
		)
	}

	if response.Shops[0].Name != "Shop APIテスト店舗" {
		t.Fatalf(
			"expected shop name %q, got %q",
			"Shop APIテスト店舗",
			response.Shops[0].Name,
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

	// 他社のcompany_idを指定してアクセス
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

	// 複数ショップ対応のレスポンス
	var response struct {
		Shops []struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
		} `json:"shops"`
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

	// 自社ショップが返っていることを確認
	if len(response.Shops) == 0 {
		t.Fatalf(
			"expected own company shop, got none: %s",
			recorder.Body.String(),
		)
	}

	// 返却されたすべてのショップを検証
	for _, returnedShop := range response.Shops {
		if returnedShop.CompanyID != user.CompanyID {
			t.Fatalf(
				"expected own company_id %d, got %d",
				user.CompanyID,
				returnedShop.CompanyID,
			)
		}

		if returnedShop.CompanyID == otherCompanyID {
			t.Fatalf(
				"another company shop must not be returned: %d",
				otherCompanyID,
			)
		}
	}
}

func TestShopsReturnsMultipleShopsForSameCompany(t *testing.T) {
	user := createTestUser(t, 1)

	// 同じ会社に2件目のショップを登録
	var secondShopID int64

	err := testDB.QueryRow(
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
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`,
		user.CompanyID,
		"追加テストショップ",
		"ツイカテストショップ",
		"03-9999-9999",
		fmt.Sprintf("additional-shop-%d@example.com", user.CompanyID),
		int16(11),
	).Scan(&secondShopID)

	if err != nil {
		t.Fatalf("failed to create second shop: %v", err)
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

	// ショップ一覧取得
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/shops",
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
		Shops []struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
		} `json:"shops"`
	}

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Shops) != 2 {
		t.Fatalf(
			"expected 2 shops, got %d: %s",
			len(response.Shops),
			recorder.Body.String(),
		)
	}

	foundSecondShop := false

	for _, returnedShop := range response.Shops {
		if returnedShop.CompanyID != user.CompanyID {
			t.Fatalf(
				"another company shop returned: company_id=%d",
				returnedShop.CompanyID,
			)
		}

		if returnedShop.ID == secondShopID {
			foundSecondShop = true
		}
	}

	if !foundSecondShop {
		t.Fatalf(
			"second shop id %d was not returned",
			secondShopID,
		)
	}
}
