package brand_test

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

// ブランドPOSTテスト用ログイン処理
func loginForBrandCreateTest(t *testing.T, email string) *http.Cookie {
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

// ブランドPOSTテスト用リクエスト
func postBrandForTest(
	t *testing.T,
	cookie *http.Cookie,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/brands",
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

// 自社ブランドの登録件数
func countBrandsForCreateTest(
	t *testing.T,
	companyID int64,
) int {
	t.Helper()

	var count int

	err := testDB.QueryRow(
		context.Background(),
		"SELECT COUNT(*) FROM brands WHERE company_id = $1",
		companyID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	return count
}

// 正常登録とDB保存、同一会社の複数登録
func TestCreateBrandSuccess(t *testing.T) {
	user := createTestUser(t, 2)
	cookie := loginForBrandCreateTest(t, user.Email)

	before := countBrandsForCreateTest(t, user.CompanyID)

	rec := postBrandForTest(t, cookie, `{
		"name": "新規テストブランド",
		"phonetic": "シンキテストブランド",
		"concept": "POST APIテスト",
		"net_shop": 1,
		"category_ids": [],
		"city_ids": [],
		"sales_recruitment_ids": [],
		"sales_style_ids": [],
		"order_ids": [],
		"pay_rule_ids": []
	}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf(
			"expected 201, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	var response struct {
		Brand struct {
			ID        int64   `json:"id"`
			CompanyID int64   `json:"company_id"`
			Name      string  `json:"name"`
			Concept   string  `json:"concept"`
			NetShop   *int64  `json:"net_shop"`
			Category  []int64 `json:"category_ids"`
			City      []int64 `json:"city_ids"`
			Approval  int64   `json:"approval"`
			Status    int64   `json:"status"`
		} `json:"brand"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	brand := response.Brand

	if brand.ID <= 0 {
		t.Fatal("new brand ID should be positive")
	}

	if brand.CompanyID != user.CompanyID {
		t.Fatalf(
			"expected company_id=%d, got %d",
			user.CompanyID,
			brand.CompanyID,
		)
	}

	if brand.Name != "新規テストブランド" {
		t.Fatalf("unexpected brand name: %s", brand.Name)
	}

	if brand.Concept != "POST APIテスト" {
		t.Fatalf("unexpected concept: %s", brand.Concept)
	}

	if brand.NetShop == nil || *brand.NetShop != 1 {
		t.Fatal("net_shop should be 1")
	}

	if brand.Category == nil || brand.City == nil {
		t.Fatal("relation IDs should be arrays, not null")
	}

	if brand.Approval != 0 || brand.Status != 0 {
		t.Fatal("approval and status should default to 0")
	}

	after := countBrandsForCreateTest(t, user.CompanyID)
	if after != before+1 {
		t.Fatalf(
			"expected %d brands, got %d",
			before+1,
			after,
		)
	}

	var savedName string
	var savedConcept string

	err := testDB.QueryRow(
		context.Background(),
		`
			SELECT name, concept
			FROM brands
			WHERE id = $1 AND company_id = $2
		`,
		brand.ID,
		user.CompanyID,
	).Scan(&savedName, &savedConcept)

	if err != nil {
		t.Fatal(err)
	}

	if savedName != brand.Name || savedConcept != brand.Concept {
		t.Fatal("saved brand values do not match response")
	}

	// 登録したブランドをGETでも取得できること
	getReq := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/v1/brands/%d", brand.ID),
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
func TestCreateBrandUnauthorized(t *testing.T) {
	rec := postBrandForTest(
		t,
		nil,
		`{"name":"未認証ブランド"}`,
	)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected 401, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

// ショップユーザーはブランドを追加できない
func TestCreateBrandForbidden(t *testing.T) {
	user := createTestUser(t, 1)
	cookie := loginForBrandCreateTest(t, user.Email)

	rec := postBrandForTest(
		t,
		cookie,
		`{"name":"権限なしブランド"}`,
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
func TestCreateBrandInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "name missing",
			body: `{}`,
		},
		{
			name: "name empty",
			body: `{"name":""}`,
		},
		{
			name: "name whitespace",
			body: `{"name":"   "}`,
		},
		{
			name: "name null",
			body: `{"name":null}`,
		},
		{
			name: "net_shop invalid",
			body: `{"name":"テスト","net_shop":5}`,
		},
		{
			name: "rate_dm_form invalid",
			body: `{"name":"テスト","rate_dm_form":6}`,
		},
		{
			name: "company_id forbidden",
			body: `{"name":"テスト","company_id":999}`,
		},
		{
			name: "approval forbidden",
			body: `{"name":"テスト","approval":1}`,
		},
		{
			name: "status forbidden",
			body: `{"name":"テスト","status":1}`,
		},
		{
			name: "duplicate categories",
			body: `{"name":"テスト","category_ids":[1,1]}`,
		},
		{
			name: "invalid category ID",
			body: `{"name":"テスト","category_ids":[0]}`,
		},
		{
			name: "null categories",
			body: `{"name":"テスト","category_ids":null}`,
		},
		{
			name: "invalid JSON",
			body: `{"name":`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := createTestUser(t, 2)
			cookie := loginForBrandCreateTest(t, user.Email)

			before := countBrandsForCreateTest(t, user.CompanyID)

			rec := postBrandForTest(t, cookie, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected 400, got %d: %s",
					rec.Code,
					rec.Body.String(),
				)
			}

			after := countBrandsForCreateTest(t, user.CompanyID)

			if after != before {
				t.Fatalf(
					"invalid request inserted brand: before=%d after=%d",
					before,
					after,
				)
			}
		})
	}
}

// 関連テーブルへのINSERT失敗時にブランド本体もロールバック
func TestCreateBrandRollback(t *testing.T) {
	user := createTestUser(t, 2)
	cookie := loginForBrandCreateTest(t, user.Email)

	before := countBrandsForCreateTest(t, user.CompanyID)

	// 存在しないカテゴリーIDを指定する。
	// brand_categories.category_idの外部キー制約で失敗させる。
	rec := postBrandForTest(t, cookie, `{
		"name": "ロールバック検証ブランド",
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

	after := countBrandsForCreateTest(t, user.CompanyID)

	if after != before {
		t.Fatalf(
			"transaction rollback failed: before=%d after=%d",
			before,
			after,
		)
	}
}
