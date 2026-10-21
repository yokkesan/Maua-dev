package shop_test

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

	"backend/internal/pricescreening"
	beego "github.com/beego/beego/v2/server/web"
)

type selectionFixture struct {
	user       testUser
	id, first  int64
	seconds    []int64
	guidelines [][3]int64
	cookie     *http.Cookie
}

func selectionExec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := testDB.Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}
func selectionID(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func newSelectionFixture(t *testing.T) *selectionFixture {
	t.Helper()
	user, id, cookie := submissionTarget(t)
	f := &selectionFixture{user: user, id: id, cookie: cookie}
	f.first = selectionID(t, "SELECT first_category_id FROM shop_price_screenings WHERE shop_id=$1", id)
	firstSecond := selectionID(t, "SELECT c.second_id FROM shop_categories ec JOIN categories c ON c.id=ec.category_id WHERE ec.shop_id=$1", id)
	t.Cleanup(func() {
		selectionExec(t, "DELETE FROM shop_price_screenings WHERE shop_id=$1", id)

		selectionExec(t, "DELETE FROM price_zone_guidelines WHERE second_id IN (SELECT id FROM second_categories WHERE first_category_id=$1) OR third_id IN (SELECT th.id FROM third_categories th JOIN second_categories s ON s.id=th.second_category_id WHERE s.first_category_id=$1)", f.first)
	})
	for i := 0; i < 4; i++ {
		second := firstSecond
		if i > 0 {
			second = selectionID(t, "INSERT INTO second_categories(first_category_id,name) VALUES ($1,$2) RETURNING id", f.first, fmt.Sprintf("selection product %d", i))
			category := selectionID(t, "INSERT INTO categories(first_id,second_id) VALUES ($1,$2) RETURNING id", f.first, second)
			selectionExec(t, "INSERT INTO shop_categories(shop_id,category_id) VALUES ($1,$2)", id, category)
		}
		f.seconds = append(f.seconds, second)
		var group [3]int64
		for rank := 1; rank <= 3; rank++ {
			group[rank-1] = selectionID(t, "INSERT INTO price_zone_guidelines(second_id,price_rank,price_range) VALUES ($1,$2,'selection fixture') RETURNING id", second, rank)
		}
		f.guidelines = append(f.guidelines, group)
	}
	return f
}
func selectionPointer(value int64) *int64 { return &value }
func selectionInput(f *selectionFixture, ranks ...int) pricescreening.SelectionInput {
	input := pricescreening.SelectionInput{FirstID: f.first, Products: []pricescreening.ProductSelection{}}
	for i, rank := range ranks {
		product := pricescreening.ProductSelection{SecondID: selectionPointer(f.seconds[i])}
		if rank > 0 {
			product.GuidelineID = selectionPointer(f.guidelines[i][rank-1])
		}
		input.Products = append(input.Products, product)
	}
	return input
}
func selectionRequest(method, id string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/shops/"+id+"/price-screening/selection", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	return rec
}
func selectionPut(t *testing.T, f *selectionFixture, input pricescreening.SelectionInput) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return selectionRequest(http.MethodPut, fmt.Sprint(f.id), f.cookie, string(data))
}
func selectionGet(t *testing.T, f *selectionFixture) pricescreening.SelectionResult {
	t.Helper()
	rec := selectionRequest(http.MethodGet, fmt.Sprint(f.id), f.cookie, "")
	requireSubmissionHTTP(t, rec, 200)
	var result pricescreening.SelectionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestSelectionSaveReadEmptyAndCalculation(t *testing.T) {
	f := newSelectionFixture(t)
	selectionExec(t, "DELETE FROM shop_price_screenings WHERE shop_id=$1", f.id)
	result := selectionGet(t, f)
	if result.Saved || result.FirstID != nil || len(result.Products) != 0 {
		t.Fatalf("initial record treated as saved: %+v", result)
	}
	for _, tc := range []struct {
		ranks []int
		zone  int64
	}{
		{[]int{1}, 1}, {[]int{2}, 2}, {[]int{3}, 3}, {[]int{1, 2}, 2}, {[]int{2, 3}, 3}, {[]int{1, 3}, 2},
		{[]int{1, 1, 2}, 1}, {[]int{1, 2, 2}, 2}, {[]int{2, 2, 3}, 2}, {[]int{2, 3, 3}, 3}, {[]int{1, 2, 3}, 2},
	} {
		input := selectionInput(f, tc.ranks...)
		requireSubmissionHTTP(t, selectionPut(t, f, input), 200)
		result = selectionGet(t, f)
		if !result.Saved || result.FirstID == nil || *result.FirstID != f.first || !reflect.DeepEqual(result.Products, input.Products) || result.PriceZoneID == nil || *result.PriceZoneID != tc.zone {
			t.Fatalf("unexpected result: %+v", result)
		}
		actual := selectionID(t, "SELECT price_zone_id FROM shops WHERE id=$1", f.id)
		if actual != tc.zone {
			t.Fatalf("zone=%d want %d", actual, tc.zone)
		}
	}
	previous := *result.PriceZoneID
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f)), 200)
	result = selectionGet(t, f)
	if !result.Saved || len(result.Products) != 0 || result.PriceZoneID == nil || *result.PriceZoneID != previous {
		t.Fatalf("empty save cleared evaluation: %+v", result)
	}
	requireSubmissionHTTP(t, submissionRequest(http.MethodPost, fmt.Sprint(f.id), f.cookie), 200)
}
func TestSelectionInvalidInputsPreserveState(t *testing.T) {
	f := newSelectionFixture(t)
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 2)), 200)
	before := selectionGet(t, f)
	state := readSubmissionSnapshot(t, testDB, f.id)
	invalid := []pricescreening.SelectionInput{
		{FirstID: 0, Products: []pricescreening.ProductSelection{}},
		{FirstID: 9223372036854775807, Products: []pricescreening.ProductSelection{}},
		{FirstID: f.first, Products: nil},
		selectionInput(f, 1, 1, 1, 1),
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(0)}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(9223372036854775807)}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(f.seconds[0]), ThirdID: selectionPointer(1)}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(f.seconds[0]), GuidelineID: selectionPointer(9223372036854775807)}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(f.seconds[0]), GuidelineID: selectionPointer(f.guidelines[1][0])}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(f.seconds[0]), GuidelineID: selectionPointer(f.guidelines[0][0])}, {SecondID: selectionPointer(f.seconds[0]), GuidelineID: selectionPointer(f.guidelines[0][1])}}},
		{FirstID: f.first, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(f.seconds[0]), GuidelineID: selectionPointer(f.guidelines[0][0])}, {SecondID: selectionPointer(f.seconds[1]), GuidelineID: selectionPointer(f.guidelines[0][0])}}},
	}
	for i, input := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) { requireSubmissionHTTP(t, selectionPut(t, f, input), 400) })
	}
	for _, body := range []string{`null`, `[]`, `{}`, `{"first_category_id":1,"products":[],"price_rank":3}`, `{} {}`, `{"first_category_id":1,"products":[{"second_category_id":1,"price_rank":3}]}`} {
		requireSubmissionHTTP(t, selectionRequest("PUT", fmt.Sprint(f.id), f.cookie, body), 400)
	}
	// Registered categories and screening products are independent selections.
	category := selectionID(t, "SELECT id FROM categories WHERE second_id=$1 AND third_id IS NULL", f.seconds[1])
	selectionExec(t, "DELETE FROM shop_categories WHERE shop_id=$1 AND category_id=$2", f.id, category)
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1, 1)), 400)
	otherFirst := selectionID(t, "INSERT INTO first_categories(name) VALUES ('selection other genre') RETURNING id")
	t.Cleanup(func() { selectionExec(t, "DELETE FROM first_categories WHERE id=$1", otherFirst) })
	mixed := selectionInput(f, 1)
	mixed.FirstID = otherFirst
	requireSubmissionHTTP(t, selectionPut(t, f, mixed), 400)
	if after := selectionGet(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid save changed selection")
	}
	if after := readSubmissionSnapshot(t, testDB, f.id); !reflect.DeepEqual(state, after) {
		t.Fatal("invalid save changed entity state")
	}
}
func TestSelectionAuthorization(t *testing.T) {
	f := newSelectionFixture(t)
	other := createTestUser(t, 1)
	wrong := createTestUser(t, 2)
	otherCookie := loginForShopCreateTest(t, other.Email)
	wrongCookie := loginForShopCreateTest(t, wrong.Email)
	body := fmt.Sprintf(`{"first_category_id":%d,"products":[]}`, f.first)
	for _, method := range []string{"GET", "PUT"} {
		for _, tc := range []struct {
			id     string
			cookie *http.Cookie
			status int
		}{
			{fmt.Sprint(f.id), nil, 401}, {fmt.Sprint(f.id), wrongCookie, 403}, {fmt.Sprint(f.id), otherCookie, 404},
			{"abc", f.cookie, 400}, {"0", f.cookie, 400}, {"-1", f.cookie, 400}, {"9223372036854775808", f.cookie, 400}, {"9223372036854775807", f.cookie, 404},
		} {
			requireSubmissionHTTP(t, selectionRequest(method, tc.id, tc.cookie, body), tc.status)
		}
	}
}
func TestSelectionSubmissionIntegrity(t *testing.T) {
	f := newSelectionFixture(t)
	before := readSubmissionSnapshot(t, testDB, f.id)
	reject := func() {
		t.Helper()
		requireSubmissionFields(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), "price_screening")
		if after := readSubmissionSnapshot(t, testDB, f.id); !reflect.DeepEqual(before, after) {
			t.Fatal("screening validation changed submitted_at")
		}
	}
	selectionExec(t, "DELETE FROM shop_price_screenings WHERE shop_id=$1", f.id)
	reject()
	// Unanswered selections can be persisted, but are never ready for submission.
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 0, 2)), 200)
	before = readSubmissionSnapshot(t, testDB, f.id)
	reject()
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1, 2)), 200)
	before = readSubmissionSnapshot(t, testDB, f.id)
	selectionExec(t, "UPDATE shop_price_screening_products SET price_zone_guideline_id=$1 WHERE shop_id=$2 AND slot_number=1", f.guidelines[1][0], f.id)
	// Duplicate answer FK is avoided here; the wrong-category guideline differs from slot 2.
	reject()
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1, 2)), 200)
	selectionExec(t, "UPDATE shop_price_screenings SET saved_at=NULL WHERE shop_id=$1", f.id)
	before = readSubmissionSnapshot(t, testDB, f.id)
	reject()
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1, 2)), 200)
	category := selectionID(t, "SELECT id FROM categories WHERE second_id=$1 AND third_id IS NULL", f.seconds[1])
	selectionExec(t, "DELETE FROM shop_categories WHERE shop_id=$1 AND category_id=$2", f.id, category)
	before = readSubmissionSnapshot(t, testDB, f.id)
	reject()
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1)), 200)
	requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), 200)
}
func TestSelectionDBConstraints(t *testing.T) {
	f := newSelectionFixture(t)
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1)), 200)
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO shop_price_screening_products(shop_id,slot_number,second_category_id) VALUES ($1,4,$2)", []any{f.id, f.seconds[1]}},
		{"INSERT INTO shop_price_screening_products(shop_id,slot_number,second_category_id) VALUES ($1,2,$2)", []any{f.id, f.seconds[0]}},
		{"INSERT INTO shop_price_screening_products(shop_id,slot_number,second_category_id,price_zone_guideline_id) VALUES ($1,2,$2,$3)", []any{f.id, f.seconds[1], f.guidelines[0][0]}},
		{"INSERT INTO shop_price_screening_products(shop_id,slot_number,second_category_id) VALUES ($1,2,9223372036854775807)", []any{f.id}},
		{"INSERT INTO shop_price_screening_products(shop_id,slot_number,second_category_id,price_zone_guideline_id) VALUES ($1,2,$2,9223372036854775807)", []any{f.id, f.seconds[1]}},
	}
	for _, statement := range statements {
		tx, err := testDB.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(context.Background(), statement.query, statement.args...)
		_ = tx.Rollback(context.Background())
		if err == nil {
			t.Fatal("invalid DB relationship accepted")
		}
	}
}
func TestSelectionSaveRollback(t *testing.T) {
	f := newSelectionFixture(t)
	requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1)), 200)
	before := selectionGet(t, f)
	state := readSubmissionSnapshot(t, testDB, f.id)
	name := fmt.Sprintf("selection_shop_fail_%d", f.id)
	selectionExec(t, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=%d THEN RAISE EXCEPTION 'selection injected internal error'; END IF; RETURN NEW; END $$`, name, f.id))
	t.Cleanup(func() { selectionExec(t, fmt.Sprintf("DROP FUNCTION %s() CASCADE", name)) })
	selectionExec(t, fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON shops FOR EACH ROW EXECUTE FUNCTION %s()", name, name))
	rec := selectionPut(t, f, selectionInput(f, 3, 3))
	requireSubmissionHTTP(t, rec, 500)
	if strings.Contains(rec.Body.String(), "injected") {
		t.Fatal("DB error leaked")
	}
	if after := selectionGet(t, f); !reflect.DeepEqual(before, after) {
		t.Fatal("failed save changed selection/evaluation")
	}
	if after := readSubmissionSnapshot(t, testDB, f.id); !reflect.DeepEqual(state, after) {
		t.Fatal("failed save changed timestamps")
	}
}

func TestSelectionConcurrentUpdates(t *testing.T) {
	f := newSelectionFixture(t)
	inputs := []pricescreening.SelectionInput{selectionInput(f, 1), selectionInput(f, 3, 3, 3)}
	bodies := make([]string, len(inputs))
	for i, input := range inputs {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		bodies[i] = string(data)
	}
	const requests = 8
	start := make(chan struct{})
	results := make(chan int, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		body := bodies[i%len(bodies)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- selectionRequest("PUT", fmt.Sprint(f.id), f.cookie, body).Code
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for code := range results {
		if code != 200 {
			t.Fatalf("concurrent save: %d", code)
		}
	}
	result := selectionGet(t, f)
	first := reflect.DeepEqual(result.Products, inputs[0].Products)
	second := reflect.DeepEqual(result.Products, inputs[1].Products)
	if !result.Saved || result.PriceZoneID == nil || (!first && !second) || (first && *result.PriceZoneID != 1) || (second && *result.PriceZoneID != 3) {
		t.Fatalf("mixed concurrent result: %+v", result)
	}
}

func TestSelectionApparelSecondAndThirdCategories(t *testing.T) {
	f := newSelectionFixture(t)
	second := selectionID(t, "INSERT INTO second_categories(first_category_id,name) VALUES (1,'selection apparel') RETURNING id")
	other := selectionID(t, "INSERT INTO second_categories(first_category_id,name) VALUES (1,'その他') RETURNING id")
	third := selectionID(t, "INSERT INTO third_categories(second_category_id,name) VALUES ($1,'selection apparel product') RETURNING id", second)
	duplicateThird := selectionID(t, "INSERT INTO third_categories(second_category_id,name) VALUES ($1,'selection apparel product') RETURNING id", second)
	cats := []int64{}
	guidelineIDs := []int64{}
	t.Cleanup(func() {
		selectionExec(t, "DELETE FROM shop_price_screenings WHERE shop_id=$1", f.id)

		selectionExec(t, "DELETE FROM price_zone_guidelines WHERE id=ANY($1::bigint[])", guidelineIDs)
		selectionExec(t, "DELETE FROM shop_categories WHERE shop_id=$1 AND category_id=ANY($2::bigint[])", f.id, cats)
		selectionExec(t, "DELETE FROM categories WHERE id=ANY($1::bigint[])", cats)
		selectionExec(t, "DELETE FROM third_categories WHERE id=ANY($1::bigint[])", []int64{third, duplicateThird})
		selectionExec(t, "DELETE FROM second_categories WHERE id=ANY($1::bigint[])", []int64{second, other})
	})
	for _, thirdID := range []int64{third, duplicateThird} {
		category := selectionID(t, "INSERT INTO categories(first_id,second_id,third_id) VALUES (1,$1,$2) RETURNING id", second, thirdID)
		cats = append(cats, category)
		selectionExec(t, "INSERT INTO shop_categories(shop_id,category_id) VALUES ($1,$2)", f.id, category)
	}
	category := selectionID(t, "INSERT INTO categories(first_id,second_id) VALUES (1,$1) RETURNING id", other)
	cats = append(cats, category)
	selectionExec(t, "INSERT INTO shop_categories(shop_id,category_id) VALUES ($1,$2)", f.id, category)
	thirdAnswer := selectionID(t, "INSERT INTO price_zone_guidelines(third_id,price_rank,price_range) VALUES ($1,3,'selection apparel') RETURNING id", third)
	otherAnswer := selectionID(t, "INSERT INTO price_zone_guidelines(second_id,price_rank,price_range) VALUES ($1,1,'selection other') RETURNING id", other)
	guidelineIDs = append(guidelineIDs, thirdAnswer, otherAnswer)
	input := pricescreening.SelectionInput{FirstID: 1, Products: []pricescreening.ProductSelection{
		{ThirdID: selectionPointer(third), GuidelineID: selectionPointer(thirdAnswer)},
		{SecondID: selectionPointer(other), GuidelineID: selectionPointer(otherAnswer)},
	}}
	requireSubmissionHTTP(t, selectionPut(t, f, input), 200)
	result := selectionGet(t, f)
	if !result.Saved || result.PriceZoneID == nil || *result.PriceZoneID != 2 || !reflect.DeepEqual(result.Products, input.Products) {
		t.Fatalf("bad apparel selection: %+v", result)
	}
	invalid := pricescreening.SelectionInput{FirstID: 1, Products: []pricescreening.ProductSelection{{SecondID: selectionPointer(second)}}}
	requireSubmissionHTTP(t, selectionPut(t, f, invalid), 400)
	invalid.Products = []pricescreening.ProductSelection{{ThirdID: selectionPointer(third)}, {ThirdID: selectionPointer(duplicateThird)}}
	requireSubmissionHTTP(t, selectionPut(t, f, invalid), 400)
	// Second/third identifiers cannot be interchanged, even when both are valid IDs.
	invalid.Products = []pricescreening.ProductSelection{{SecondID: selectionPointer(other), GuidelineID: selectionPointer(thirdAnswer)}}
	requireSubmissionHTTP(t, selectionPut(t, f, invalid), 400)
}
