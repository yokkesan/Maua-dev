package brand_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"backend/internal/pricescreening"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type screeningFixture struct {
	user        testUser
	brandID     int64
	firstID     int64
	secondIDs   []int64
	thirdIDs    []int64
	categoryIDs []int64
	ids         [][3]int64
	cookie      *http.Cookie
}

func screeningSQL(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := testDB.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func screeningID(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func newScreeningFixture(t *testing.T) *screeningFixture {
	t.Helper()
	f := &screeningFixture{user: createTestUser(t, 2)}
	f.brandID = screeningID(t, `SELECT id FROM brands WHERE company_id=$1`, f.user.CompanyID)
	f.cookie = loginForBrandCreateTest(t, f.user.Email)
	f.firstID = screeningID(t, `INSERT INTO first_categories(name) VALUES ('No9 test genre') RETURNING id`)
	t.Cleanup(func() {
		screeningSQL(t, `DELETE FROM companies WHERE id=$1`, f.user.CompanyID)
		var ids []int64
		for _, group := range f.ids {
			ids = append(ids, group[:]...)
		}
		screeningSQL(t, `DELETE FROM price_zone_guidelines WHERE id=ANY($1::bigint[])`, ids)
		screeningSQL(t, `DELETE FROM categories WHERE id=ANY($1::bigint[])`, f.categoryIDs)
		screeningSQL(t, `DELETE FROM third_categories WHERE id=ANY($1::bigint[])`, f.thirdIDs)
		screeningSQL(t, `DELETE FROM second_categories WHERE id=ANY($1::bigint[])`, f.secondIDs)
		screeningSQL(t, `DELETE FROM first_categories WHERE id=$1`, f.firstID)
	})
	for i := 0; i < 4; i++ {
		second := screeningID(t, `INSERT INTO second_categories(first_category_id,name) VALUES ($1,$2) RETURNING id`, f.firstID, fmt.Sprintf("product %d", i))
		f.secondIDs = append(f.secondIDs, second)
		cat := screeningID(t, `INSERT INTO categories(first_id,second_id) VALUES ($1,$2) RETURNING id`, f.firstID, second)
		f.categoryIDs = append(f.categoryIDs, cat)
		screeningSQL(t, `INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)`, f.brandID, cat)
		var group [3]int64
		for rank := 1; rank <= 3; rank++ {
			group[rank-1] = screeningID(t, `INSERT INTO price_zone_guidelines(second_id,price_rank,price_range) VALUES ($1,$2,'test') RETURNING id`, second, rank)
		}
		f.ids = append(f.ids, group)
	}
	return f
}

func screeningRequest(t *testing.T, method string, id string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/brands/"+id+"/price-screening", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	return rec
}

func screeningPut(t *testing.T, f *screeningFixture, ids []int64) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{"price_zone_guideline_ids": ids})
	if err != nil {
		t.Fatal(err)
	}
	return screeningRequest(t, http.MethodPut, fmt.Sprint(f.brandID), f.cookie, string(body))
}

func assertScreening(t *testing.T, rec *httptest.ResponseRecorder, ids []int64, zone *int64) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result pricescreening.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := append([]int64{}, ids...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !reflect.DeepEqual(result.GuidelineIDs, want) || !reflect.DeepEqual(result.PriceZoneID, zone) {
		t.Fatalf("got %+v, want ids=%v zone=%v", result, want, zone)
	}
}

func assertScreeningDB(t *testing.T, f *screeningFixture, ids []int64, zone int64) {
	t.Helper()
	var saved []int64
	var actual int64
	err := testDB.QueryRow(context.Background(), `SELECT ARRAY(SELECT price_zone_guideline_id FROM brand_price_zone_guidelines WHERE brand_id=$1 ORDER BY price_zone_guideline_id),price_zone_id FROM brands WHERE id=$1`, f.brandID).Scan(&saved, &actual)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]int64{}, ids...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !reflect.DeepEqual(saved, want) || actual != zone {
		t.Fatalf("DB ids=%v zone=%d, want %v %d", saved, actual, want, zone)
	}
}

func TestPriceScreeningSaveReadReplaceAndRound(t *testing.T) {
	f := newScreeningFixture(t)
	assertScreening(t, screeningRequest(t, "GET", fmt.Sprint(f.brandID), f.cookie, ""), []int64{}, nil)
	assertScreening(t, screeningPut(t, f, []int64{}), []int64{}, nil)
	for _, tc := range []struct {
		ranks []int
		zone  int64
	}{
		{[]int{1}, 1}, {[]int{2}, 2}, {[]int{3}, 3},
		{[]int{1, 2}, 2}, {[]int{2, 3}, 3}, {[]int{1, 3}, 2},
		{[]int{1, 1, 2}, 1}, {[]int{1, 2, 2}, 2}, {[]int{2, 2, 3}, 2}, {[]int{2, 3, 3}, 3}, {[]int{1, 2, 3}, 2},
	} {
		t.Run(fmt.Sprint(tc.ranks), func(t *testing.T) {
			var ids []int64
			for i, rank := range tc.ranks {
				ids = append(ids, f.ids[i][rank-1])
			}
			assertScreening(t, screeningPut(t, f, ids), ids, &tc.zone)
			assertScreeningDB(t, f, ids, tc.zone)
			assertScreening(t, screeningRequest(t, "GET", fmt.Sprint(f.brandID), f.cookie, ""), ids, &tc.zone)
			// 同じPUTの再実行でも重複せず、空配列も既存結果を保持する。
			assertScreening(t, screeningPut(t, f, ids), ids, &tc.zone)
			assertScreening(t, screeningPut(t, f, []int64{}), ids, &tc.zone)
			assertScreeningDB(t, f, ids, tc.zone)
		})
	}
}

func TestPriceScreeningInvalidMasterPreservesResult(t *testing.T) {
	f := newScreeningFixture(t)
	ids := []int64{f.ids[0][0]}
	zone := int64(1)
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	// 専用の1接続プール上の一時テーブルで不正マスターを再現する。
	// publicの制約・マスターデータを変更せず、実Repositoryの検証を通す。
	config := testDB.Config()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(context.Background(), query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TEMP TABLE price_zone_guidelines AS SELECT * FROM public.price_zone_guidelines WHERE id=ANY($1::bigint[])`, []int64{f.ids[1][2]})
	exec(`CREATE TEMP TABLE price_zones AS SELECT * FROM public.price_zones`)
	service := pricescreening.NewService(pricescreening.NewPostgresRepository(pool))
	for _, tc := range []struct{ name, mutate, restore string }{
		{"invalid rank", `UPDATE price_zone_guidelines SET price_rank=4`, `UPDATE price_zone_guidelines SET price_rank=3`},
		{"missing category", `UPDATE price_zone_guidelines SET second_id=NULL`, fmt.Sprintf(`UPDATE price_zone_guidelines SET second_id=%d`, f.secondIDs[1])},
		{"both category levels", `UPDATE price_zone_guidelines SET third_id=1`, `UPDATE price_zone_guidelines SET third_id=NULL`},
		{"missing zone", `DELETE FROM price_zones WHERE id=3`, `INSERT INTO price_zones SELECT * FROM public.price_zones WHERE id=3`},
		{"wrong zone mapping", `UPDATE price_zones SET name='Standard' WHERE id=3`, `UPDATE price_zones SET name='Luxury' WHERE id=3`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec(tc.mutate)
			_, err := service.Put(context.Background(), f.brandID, f.user.CompanyID, []int64{f.ids[1][2]})
			if !errors.Is(err, pricescreening.ErrInvalidMaster) {
				t.Fatalf("got %v, want master error", err)
			}
			assertScreeningDB(t, f, ids, zone)
			exec(tc.restore)
		})
	}
}

func TestPriceScreeningApparelCategories(t *testing.T) {
	f := newScreeningFixture(t)
	secondID := screeningID(t, `INSERT INTO second_categories(first_category_id,name) VALUES (1,'No9 apparel') RETURNING id`)
	f.secondIDs = append(f.secondIDs, secondID)
	thirdID := screeningID(t, `INSERT INTO third_categories(second_category_id,name) VALUES ($1,'No9 apparel product') RETURNING id`, secondID)
	f.thirdIDs = append(f.thirdIDs, thirdID)
	category := screeningID(t, `INSERT INTO categories(first_id,second_id,third_id) VALUES (1,$1,$2) RETURNING id`, secondID, thirdID)
	f.categoryIDs = append(f.categoryIDs, category)
	screeningSQL(t, `INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)`, f.brandID, category)
	var group [3]int64
	for rank := 1; rank <= 3; rank++ {
		group[rank-1] = screeningID(t, `INSERT INTO price_zone_guidelines(third_id,price_rank,price_range) VALUES ($1,$2,'test') RETURNING id`, thirdID, rank)
	}
	f.ids = append(f.ids, group)
	guideline := group[2]
	zone := int64(3)
	assertScreening(t, screeningPut(t, f, []int64{guideline}), []int64{guideline}, &zone)
	// ID 8以外でも、第二階層名が「その他」なら第二階層で回答する。
	otherSecond := screeningID(t, `INSERT INTO second_categories(first_category_id,name) VALUES (1,'その他') RETURNING id`)
	f.secondIDs = append(f.secondIDs, otherSecond)
	otherCategory := screeningID(t, `INSERT INTO categories(first_id,second_id) VALUES (1,$1) RETURNING id`, otherSecond)
	f.categoryIDs = append(f.categoryIDs, otherCategory)
	var otherGroup [3]int64
	for rank := 1; rank <= 3; rank++ {
		otherGroup[rank-1] = screeningID(t, `INSERT INTO price_zone_guidelines(second_id,price_rank,price_range) VALUES ($1,$2,'test') RETURNING id`, otherSecond, rank)
	}
	f.ids = append(f.ids, otherGroup)
	other := otherGroup[0]
	screeningSQL(t, `INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)`, f.brandID, otherCategory)
	zone = 2
	assertScreening(t, screeningPut(t, f, []int64{guideline, other}), []int64{guideline, other}, &zone)
}

func TestPriceScreeningPatchConsistency(t *testing.T) {
	f := newScreeningFixture(t)
	patch := func(body string, status int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/brands/%d", f.brandID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(f.cookie)
		rec := httptest.NewRecorder()
		beego.BeeApp.Handlers.ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("PATCH %s: got %d, want %d: %s", body, rec.Code, status, rec.Body.String())
		}
	}
	assertCategories := func(want []int64) {
		t.Helper()
		var actual []int64
		if err := testDB.QueryRow(context.Background(), `SELECT ARRAY(
			SELECT category_id FROM brand_categories WHERE brand_id=$1 ORDER BY category_id
		)`, f.brandID).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		want = append([]int64{}, want...)
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("categories=%v, want %v", actual, want)
		}
	}
	assertConcept := func(want string) {
		t.Helper()
		var actual string
		if err := testDB.QueryRow(context.Background(), `SELECT concept FROM brands WHERE id=$1`, f.brandID).Scan(&actual); err != nil {
			t.Fatal(err)
		}
		if actual != want {
			t.Fatalf("concept=%q, want %q", actual, want)
		}
	}

	t.Run("price zone before answers", func(t *testing.T) {
		patch(`{"price_zone_id":3,"concept":"before screening"}`, 200)
		assertScreeningDB(t, f, []int64{}, 3)
		patch(`{"price_zone_id":null}`, 200)
		assertScreening(t, screeningRequest(t, "GET", fmt.Sprint(f.brandID), f.cookie, ""), []int64{}, nil)
	})
	ids := []int64{f.ids[0][1], f.ids[1][1]}
	zone := int64(2)
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	for _, tc := range []struct{ name, body string }{
		{"different price zone", `{"price_zone_id":1}`},
		{"same price zone", `{"price_zone_id":2}`},
		{"null price zone", `{"price_zone_id":null}`},
		{"remove all categories", `{"category_ids":[]}`},
		{"remove one answered category", fmt.Sprintf(`{"category_ids":[%d,%d]}`, f.categoryIDs[0], f.categoryIDs[2])},
		{"combined field update", fmt.Sprintf(`{"concept":"must not persist","price_zone_id":3,"category_ids":[%d,%d]}`, f.categoryIDs[0], f.categoryIDs[1])},
		{"combined invalid categories", fmt.Sprintf(`{"concept":"must not persist","category_ids":[%d]}`, f.categoryIDs[0])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patch(tc.body, 400)
			assertScreeningDB(t, f, ids, zone)
			assertCategories(f.categoryIDs)
			assertConcept("before screening")
		})
	}
	t.Run("category change preserving all answers", func(t *testing.T) {
		kept := []int64{f.categoryIDs[0], f.categoryIDs[1], f.categoryIDs[3]}
		patch(fmt.Sprintf(`{"category_ids":[%d,%d,%d]}`, kept[0], kept[1], kept[2]), 200)
		assertCategories(kept)
		assertScreeningDB(t, f, ids, zone)
	})
	t.Run("ordinary patch", func(t *testing.T) {
		patch(`{"concept":"after screening","price_min":"9999999","price_max":"1"}`, 200)
		assertConcept("after screening")
		assertCategories([]int64{f.categoryIDs[0], f.categoryIDs[1], f.categoryIDs[3]})
		assertScreeningDB(t, f, ids, zone)
	})
}

func TestPriceScreeningInvalidInputPreservesResult(t *testing.T) {
	f := newScreeningFixture(t)
	ids := []int64{f.ids[0][1]}
	zone := int64(2)
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"price_zone_guideline_ids":null}`, `{"price_zone_guideline_ids":"1"}`,
		`{"price_zone_guideline_ids":[0]}`, `{"price_zone_guideline_ids":[-1]}`, `{"price_zone_guideline_ids":[1.5]}`,
		`{"price_zone_guideline_ids":["1"]}`, `{"price_zone_guideline_ids":[9223372036854775807]}`,
		fmt.Sprintf(`{"price_zone_guideline_ids":[%d,%d]}`, ids[0], ids[0]),
		fmt.Sprintf(`{"price_zone_guideline_ids":[%d,%d]}`, f.ids[0][0], f.ids[0][2]),
		fmt.Sprintf(`{"price_zone_guideline_ids":[%d,%d,%d,%d]}`, f.ids[0][0], f.ids[1][0], f.ids[2][0], f.ids[3][0]),
		`{"price_zone_guideline_ids":[],"price_rank":3}`, `{"price_zone_guideline_ids":[]} {}`,
		`{"price_zone_guideline_ids":[`,
	} {
		t.Run(body, func(t *testing.T) {
			rec := screeningRequest(t, "PUT", fmt.Sprint(f.brandID), f.cookie, body)
			if rec.Code != 400 {
				t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
			}
			assertScreeningDB(t, f, ids, zone)
		})
	}
	// 同じジャンルでも、ブランドで選択されていないカテゴリーは拒否する。
	screeningSQL(t, `DELETE FROM brand_categories WHERE brand_id=$1 AND category_id=$2`, f.brandID, f.categoryIDs[1])
	if rec := screeningPut(t, f, []int64{f.ids[1][0]}); rec.Code != 400 {
		t.Fatalf("unselected category: %d %s", rec.Code, rec.Body.String())
	}
	// ブランドに両方登録されていても、異なるジャンルの混在は拒否する。
	other := newScreeningFixture(t)
	screeningSQL(t, `INSERT INTO brand_categories(brand_id,category_id) VALUES ($1,$2)`, f.brandID, other.categoryIDs[0])
	t.Cleanup(func() {
		screeningSQL(t, `DELETE FROM brand_categories WHERE brand_id=$1 AND category_id=$2`, f.brandID, other.categoryIDs[0])
	})
	if rec := screeningPut(t, f, []int64{ids[0], other.ids[0][1]}); rec.Code != 400 {
		t.Fatalf("mixed genre: %d %s", rec.Code, rec.Body.String())
	}
	assertScreeningDB(t, f, ids, zone)
}

func TestPriceScreeningAuthorization(t *testing.T) {
	f := newScreeningFixture(t)
	other := createTestUser(t, 2)
	otherCookie := loginForBrandCreateTest(t, other.Email)
	shop := createTestUser(t, 1)
	shopCookie := loginForBrandCreateTest(t, shop.Email)
	for _, method := range []string{"GET", "PUT"} {
		for _, tc := range []struct {
			name, id string
			cookie   *http.Cookie
			status   int
		}{
			{"anonymous", fmt.Sprint(f.brandID), nil, 401},
			{"shop", fmt.Sprint(f.brandID), shopCookie, 403},
			{"other company", fmt.Sprint(f.brandID), otherCookie, 404},
			{"missing", "9223372036854775807", f.cookie, 404},
			{"invalid", "abc", f.cookie, 400}, {"zero", "0", f.cookie, 400},
		} {
			t.Run(method+tc.name, func(t *testing.T) {
				rec := screeningRequest(t, method, tc.id, tc.cookie, `{"price_zone_guideline_ids":[]}`)
				if rec.Code != tc.status {
					t.Fatalf("expected %d got %d: %s", tc.status, rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestPriceScreeningDBFailureRollsBack(t *testing.T) {
	f := newScreeningFixture(t)
	ids := []int64{f.ids[0][0]}
	zone := int64(1)
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	// ブランド限定のトリガーで、回答DELETE/INSERT後の価格帯UPDATEを失敗させる。
	name := fmt.Sprintf("no9_fail_%d", f.brandID)
	screeningSQL(t, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=%d THEN RAISE EXCEPTION 'no9 injected update failure'; END IF; RETURN NEW; END $$`, name, f.brandID))
	t.Cleanup(func() { screeningSQL(t, fmt.Sprintf(`DROP FUNCTION %s() CASCADE`, name)) })
	screeningSQL(t, fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON brands FOR EACH ROW EXECUTE FUNCTION %s()`, name, name))
	rec := screeningPut(t, f, []int64{f.ids[1][2], f.ids[2][2]})
	if rec.Code != 500 {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "injected") {
		t.Fatal("DB error leaked")
	}
	assertScreeningDB(t, f, ids, zone)
}

func TestPriceScreeningExistingBrandAPIRegression(t *testing.T) {
	f := newScreeningFixture(t)
	zone := int64(3)
	ids := []int64{f.ids[0][2]}
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	for _, path := range []string{"/api/v1/brands", fmt.Sprintf("/api/v1/brands/%d", f.brandID)} {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(f.cookie)
		rec := httptest.NewRecorder()
		beego.BeeApp.Handlers.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
		}
		if path != "/api/v1/brands" {
			var response struct {
				Brand map[string]any `json:"brand"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Brand["price_zone_id"] != float64(3) {
				t.Fatalf("unexpected detail %v", response.Brand)
			}
			if _, ok := response.Brand["price_zone_guideline_ids"]; ok {
				t.Fatal("existing detail format changed")
			}
		}
	}
	// 価格文字列は算出に使わず、通常のPATCHでも回答を変更しない。
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/brands/%d", f.brandID), strings.NewReader(`{"concept":"No9 regression","price_min":"9999999","price_max":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(f.cookie)
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body.String())
	}
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	assertScreeningDB(t, f, ids, zone)
	// 既存POSTではprice_zone_idの直接指定も引き続き可能。
	rec = postBrandForTest(t, f.cookie, `{"name":"No9 compatibility","price_zone_id":2}`)
	if rec.Code != 201 {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPriceScreeningDBConstraints(t *testing.T) {
	f := newScreeningFixture(t)
	ids := []int64{f.ids[0][0]}
	zone := int64(1)
	assertScreening(t, screeningPut(t, f, ids), ids, &zone)
	for _, tc := range []struct {
		name                 string
		brandID, guidelineID int64
		code                 string
	}{
		{"duplicate", f.brandID, ids[0], "23505"},
		{"missing guideline", f.brandID, 9223372036854775807, "23503"},
		{"missing brand", 9223372036854775807, ids[0], "23503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := testDB.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			_, err = tx.Exec(context.Background(), `INSERT INTO brand_price_zone_guidelines(brand_id,price_zone_guideline_id) VALUES ($1,$2)`, tc.brandID, tc.guidelineID)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
				t.Fatalf("got %v, want %s", err, tc.code)
			}
		})
	}
	assertScreeningDB(t, f, ids, zone)
}
