package shop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

// ショップPOSTテスト用ログイン処理
func loginForShopCreateTest(t *testing.T, email string) *http.Cookie {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": testPassword,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/login",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"login failed: status=%d body=%s",
			rec.Code,
			rec.Body.String(),
		)
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "auma_session" {
			return cookie
		}
	}

	t.Fatal("auma_session cookie not found")
	return nil
}

// ショップPOSTテスト用リクエスト
func postShopForTest(
	t *testing.T,
	cookie *http.Cookie,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/shops",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)

	return rec
}

// 自社ショップの登録件数
func countShopsForCreateTest(
	t *testing.T,
	companyID int64,
) int {
	t.Helper()

	var count int

	err := testDB.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM shops WHERE company_id = $1",
		companyID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	return count
}

// 正常登録とDB保存、同一会社の複数登録
func TestCreateShopSuccess(t *testing.T) {
	user := createTestUser(t, 1)
	cookie := loginForShopCreateTest(t, user.Email)

	before := countShopsForCreateTest(t, user.CompanyID)

	rec := postShopForTest(t, cookie, `{
		"name": "新規テストショップ",
		"phonetic": "シンキテストショップ",
		"registration_plan": 11,
		"concept": "ショップPOST APIテスト",
		"net_shop": 1,
		"address": "大分県大分市",
		"category_ids": []
	}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"expected 201, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var response struct {
		Shop struct {
			ID               int64   `json:"id"`
			CompanyID        int64   `json:"company_id"`
			Name             string  `json:"name"`
			Concept          string  `json:"concept"`
			Address          string  `json:"address"`
			RegistrationPlan int64   `json:"registration_plan"`
			NetShop          *int64  `json:"net_shop"`
			Category         []int64 `json:"category_ids"`
			Approval         int64   `json:"approval"`
			Status           int64   `json:"status"`
		} `json:"shop"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	shop := response.Shop

	if shop.ID <= 0 {
		t.Fatal("new shop ID should be positive")
	}

	if shop.CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id=%d, got %d",
			user.CompanyID,
			shop.CompanyID,
		)
	}

	if shop.Name != "新規テストショップ" {
		t.Fatalf("unexpected shop name: %s", shop.Name)
	}

	if shop.Concept != "ショップPOST APIテスト" {
		t.Fatalf("unexpected concept: %s", shop.Concept)
	}

	if shop.Address != "大分県大分市" {
		t.Fatalf("unexpected address: %s", shop.Address)
	}

	if shop.RegistrationPlan != 11 {
		t.Fatalf(
			"expected registration_plan=11, got %d",
			shop.RegistrationPlan,
		)
	}

	if shop.NetShop == nil || *shop.NetShop != 1 {
		t.Fatal("net_shop should be 1")
	}

	if shop.Category == nil {
		t.Fatal("category_ids should be an array, not null")
	}

	if shop.Approval != 0 || shop.Status != 0 {
		t.Fatal("approval and status should default to 0")
	}

	after := countShopsForCreateTest(t, user.CompanyID)

	if after != before+1 {
		t.Fatalf(
			"expected %d shops, got %d",
			before+1,
			after,
		)
	}

	var savedName string
	var savedConcept string
	var savedPlan int64

	err := testDB.QueryRow(
		context.Background(),
		`
			SELECT name, concept, registration_plan
			FROM shops
			WHERE id = $1 AND company_id = $2
		`,
		shop.ID,
		user.CompanyID,
	).Scan(&savedName, &savedConcept, &savedPlan)

	if err != nil {
		t.Fatal(err)
	}

	if savedName != shop.Name ||
		savedConcept != shop.Concept ||
		savedPlan != shop.RegistrationPlan {
		t.Fatal("saved shop values do not match response")
	}

	// 登録後のGETでも取得できること
	getReq := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/v1/shops/%d", shop.ID),
		nil,
	)
	getReq.AddCookie(cookie)

	getRec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf(
			"GET after POST: expected 200, got %d: %s",
			getRec.Code,
			getRec.Body.String(),
		)
	}
}

// 未認証
func TestCreateShopUnauthorized(t *testing.T) {
	rec := postShopForTest(
		t,
		nil,
		`{"name":"未認証ショップ","registration_plan":11}`,
	)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected 401, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

// ブランドユーザーはショップを追加できない
func TestCreateShopForbidden(t *testing.T) {
	user := createTestUser(t, 2)
	cookie := loginForShopCreateTest(t, user.Email)

	rec := postShopForTest(
		t,
		cookie,
		`{"name":"権限なしショップ","registration_plan":11}`,
	)

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"expected 403, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

// 不正な入力
func TestCreateShopInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "name missing",
			body: `{"registration_plan":11}`,
		},
		{
			name: "name empty",
			body: `{"name":"","registration_plan":11}`,
		},
		{
			name: "name whitespace",
			body: `{"name":"   ","registration_plan":11}`,
		},
		{
			name: "name null",
			body: `{"name":null,"registration_plan":11}`,
		},
		{
			name: "registration_plan missing",
			body: `{"name":"テスト"}`,
		},
		{
			name: "registration_plan 0",
			body: `{"name":"テスト","registration_plan":0}`,
		},
		{
			name: "registration_plan 12",
			body: `{"name":"テスト","registration_plan":12}`,
		},
		{
			name: "registration_plan null",
			body: `{"name":"テスト","registration_plan":null}`,
		},
		{
			name: "net_shop invalid",
			body: `{"name":"テスト","registration_plan":11,"net_shop":5}`,
		},
		{
			name: "company_id forbidden",
			body: `{"name":"テスト","registration_plan":11,"company_id":999}`,
		},
		{
			name: "approval forbidden",
			body: `{"name":"テスト","registration_plan":11,"approval":1}`,
		},
		{
			name: "status forbidden",
			body: `{"name":"テスト","registration_plan":11,"status":1}`,
		},
		{
			name: "duplicate categories",
			body: `{"name":"テスト","registration_plan":11,"category_ids":[1,1]}`,
		},
		{
			name: "invalid category ID",
			body: `{"name":"テスト","registration_plan":11,"category_ids":[0]}`,
		},
		{
			name: "null categories",
			body: `{"name":"テスト","registration_plan":11,"category_ids":null}`,
		},
		{
			name: "invalid JSON",
			body: `{"name":`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := createTestUser(t, 1)
			cookie := loginForShopCreateTest(t, user.Email)

			before := countShopsForCreateTest(t, user.CompanyID)

			rec := postShopForTest(t, cookie, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected 400, got %d: %s",
					rec.Code,
					rec.Body.String(),
				)
			}

			after := countShopsForCreateTest(t, user.CompanyID)

			if after != before {
				t.Fatalf(
					"invalid request inserted shop: before=%d after=%d",
					before,
					after,
				)
			}
		})
	}
}

// カテゴリー登録失敗時にショップ本体もロールバック
func TestCreateShopRollback(t *testing.T) {
	user := createTestUser(t, 1)
	cookie := loginForShopCreateTest(t, user.Email)

	before := countShopsForCreateTest(t, user.CompanyID)

	rec := postShopForTest(t, cookie, `{
		"name": "ロールバック検証ショップ",
		"registration_plan": 11,
		"concept": "保存されてはいけない",
		"category_ids": [9223372036854775807]
	}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected 400, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	after := countShopsForCreateTest(t, user.CompanyID)

	if after != before {
		t.Fatalf(
			"transaction rollback failed: before=%d after=%d",
			before,
			after,
		)
	}
}
