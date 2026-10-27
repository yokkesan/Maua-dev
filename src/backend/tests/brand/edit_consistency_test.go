package brand_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

func editRequest(f *selectionFixture, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/brands/%d", f.id), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(f.cookie)
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	return rec
}

// Snapshot every target field and all screening/category rows to prove rollback,
// including timestamps and the legacy answer mirror where present.
func editSnapshot(t *testing.T, f *selectionFixture) string {
	t.Helper()
	var snapshot string
	err := testDB.QueryRow(context.Background(), `SELECT jsonb_build_array(
  to_jsonb(e),
  (SELECT jsonb_agg(to_jsonb(c) ORDER BY category_id) FROM brand_categories c WHERE brand_id=e.id),
  (SELECT to_jsonb(h) FROM brand_price_screenings h WHERE brand_id=e.id),
  (SELECT jsonb_agg(to_jsonb(p) ORDER BY slot_number) FROM brand_price_screening_products p WHERE brand_id=e.id)
  , (SELECT jsonb_agg(to_jsonb(a) ORDER BY price_zone_guideline_id) FROM brand_price_zone_guidelines a WHERE brand_id=e.id)
 )::text FROM brands e WHERE e.id=$1`, f.id).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestEditScreeningConsistency(t *testing.T) {
	for _, submitted := range []bool{false, true} {
		t.Run(fmt.Sprintf("submitted_%t", submitted), func(t *testing.T) {
			f := newSelectionFixture(t)
			requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 2)), 200)
			if submitted {
				requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), 200)
			}
			// Incomplete answers remain editable both before and after submission.
			requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 2, 0)), 200)
			state := readSubmissionSnapshot(t, testDB, f.id).State
			category := selectionID(t, "SELECT id FROM categories WHERE second_id=$1 AND third_id IS NULL", f.seconds[0])
			for _, body := range []string{
				`{"concept":"rollback","price_zone_id":3}`,
				`{"concept":"rollback","price_zone_id":null}`,
				`{"concept":"rollback","category_ids":[]}`,
				fmt.Sprintf(`{"concept":"rollback","category_ids":[%d]}`, category), // loses unanswered product
			} {
				before := editSnapshot(t, f)
				requireSubmissionHTTP(t, editRequest(f, body), 400)
				if after := editSnapshot(t, f); before != after {
					t.Fatal("failed PATCH changed DB")
				}
			}
			before := editSnapshot(t, f)
			invalid := selectionInput(f, 1)
			invalid.Products[0].GuidelineID = selectionPointer(f.guidelines[1][0])
			requireSubmissionHTTP(t, selectionPut(t, f, invalid), 400)
			if editSnapshot(t, f) != before {
				t.Fatal("invalid PUT changed DB")
			}
			secondCategory := selectionID(t, "SELECT id FROM categories WHERE second_id=$1 AND third_id IS NULL", f.seconds[1])
			requireSubmissionHTTP(t, editRequest(f, fmt.Sprintf(`{"concept":"","category_ids":[%d,%d]}`, category, secondCategory)), 200)
			requireSubmissionHTTP(t, editRequest(f, `{"concept":null,"target_min":""}`), 200)
			if after := readSubmissionSnapshot(t, testDB, f.id).State; !reflect.DeepEqual(state, after) {
				t.Fatal("PATCH changed submission state")
			}
			requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 3, 0)), 200)
			if after := readSubmissionSnapshot(t, testDB, f.id).State; !reflect.DeepEqual(state, after) {
				t.Fatal("PUT changed submission state")
			}
		})
	}
}

func TestEditEmptySelectionAllowsCategoryClear(t *testing.T) {
	f := newSelectionFixture(t)
	requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), 200)
	state := readSubmissionSnapshot(t, testDB, f.id).State
	requireSubmissionHTTP(t, editRequest(f, `{"category_ids":[],"concept":""}`), 200)
	if after := readSubmissionSnapshot(t, testDB, f.id).State; !reflect.DeepEqual(state, after) {
		t.Fatal("category clear changed submission state")
	}
}

func TestEditConcurrentCategoryAndSelection(t *testing.T) {
	for _, submitted := range []bool{false, true} {
		t.Run(fmt.Sprint(submitted), func(t *testing.T) {
			f := newSelectionFixture(t)
			requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 1)), 200)
			if submitted {
				requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), 200)
			}
			state := readSubmissionSnapshot(t, testDB, f.id).State
			category := selectionID(t, "SELECT id FROM categories WHERE second_id=$1 AND third_id IS NULL", f.seconds[0])
			start := make(chan struct{})
			results := make(chan int, 2)
			go func() { <-start; results <- editRequest(f, fmt.Sprintf(`{"category_ids":[%d]}`, category)).Code }()
			// Serializing these two operations must allow exactly one winner.
			input := selectionInput(f, 1, 2)
			data := fmt.Sprintf(`{"first_category_id":%d,"products":[{"second_category_id":%d,"price_zone_guideline_id":%d},{"second_category_id":%d,"price_zone_guideline_id":%d}]}`, f.first, *input.Products[0].SecondID, *input.Products[0].GuidelineID, *input.Products[1].SecondID, *input.Products[1].GuidelineID)
			go func() { <-start; results <- selectionRequest("PUT", fmt.Sprint(f.id), f.cookie, data).Code }()
			close(start)
			a, b := <-results, <-results
			if !((a == 200 && b == 400) || (a == 400 && b == 200)) {
				t.Fatalf("concurrent results %d %d", a, b)
			}
			result := selectionGet(t, f)
			var categories int
			if err := testDB.QueryRow(context.Background(), "SELECT count(*) FROM brand_categories WHERE brand_id=$1", f.id).Scan(&categories); err != nil {
				t.Fatal(err)
			}
			if (categories == 1 && len(result.Products) != 1) || (categories > 1 && len(result.Products) != 2) {
				t.Fatal("mixed concurrent state")
			}
			if after := readSubmissionSnapshot(t, testDB, f.id).State; !reflect.DeepEqual(state, after) {
				t.Fatal("concurrent edits changed submission state")
			}
		})
	}
}

func TestEditCategoryHierarchy(t *testing.T) {
	f := newSelectionFixture(t)
	third := selectionID(t, "INSERT INTO third_categories(second_category_id,name) VALUES ($1,'edit hierarchy') RETURNING id", f.seconds[1])
	second := selectionID(t, "INSERT INTO second_categories(first_category_id,name) VALUES ($1,'edit genre mismatch') RETURNING id", f.first)
	bad := []int64{
		selectionID(t, "INSERT INTO categories(first_id,second_id) VALUES (1,$1) RETURNING id", second),
		selectionID(t, "INSERT INTO categories(first_id,second_id,third_id) VALUES ($1,$2,$3) RETURNING id", f.first, f.seconds[0], third),
	}
	t.Cleanup(func() {
		selectionExec(t, "DELETE FROM categories WHERE id=ANY($1::bigint[])", bad)
		selectionExec(t, "DELETE FROM third_categories WHERE id=$1", third)
		selectionExec(t, "DELETE FROM second_categories WHERE id=$1", second)
	})
	for _, category := range bad {
		before := editSnapshot(t, f)
		requireSubmissionHTTP(t, editRequest(f, fmt.Sprintf(`{"concept":"rollback hierarchy","category_ids":[%d]}`, category)), 400)
		if editSnapshot(t, f) != before {
			t.Fatal("invalid hierarchy changed DB")
		}
	}
}

func TestEditLegacyPUTPreservesProductSelection(t *testing.T) {
	for _, submitted := range []bool{false, true} {
		t.Run(fmt.Sprint(submitted), func(t *testing.T) {
			f := newSelectionFixture(t)
			requireSubmissionHTTP(t, selectionPut(t, f, selectionInput(f, 2, 2)), 200)
			if submitted {
				requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(f.id), f.cookie), 200)
			}
			state := readSubmissionSnapshot(t, testDB, f.id).State
			before := editSnapshot(t, f)
			body := fmt.Sprintf(`{"price_zone_guideline_ids":[%d]}`, f.guidelines[2][0])
			requireSubmissionHTTP(t, screeningRequest(t, "PUT", fmt.Sprint(f.id), f.cookie, body), 400)
			if editSnapshot(t, f) != before {
				t.Fatal("rejected legacy PUT changed DB")
			}
			requireSubmissionHTTP(t, screeningRequest(t, "PUT", fmt.Sprint(f.id), f.cookie, `{"price_zone_guideline_ids":[]}`), 200)
			if editSnapshot(t, f) != before {
				t.Fatal("empty legacy PUT changed DB")
			}
			body = fmt.Sprintf(`{"price_zone_guideline_ids":[%d]}`, f.guidelines[1][2])
			requireSubmissionHTTP(t, screeningRequest(t, "PUT", fmt.Sprint(f.id), f.cookie, body), 200)
			result := selectionGet(t, f)
			if result.Saved || !reflect.DeepEqual(result.Products, selectionInput(f, 0, 3).Products) || result.PriceZoneID == nil || *result.PriceZoneID != 3 {
				t.Fatalf("inconsistent legacy save: %+v", result)
			}
			if after := readSubmissionSnapshot(t, testDB, f.id).State; !reflect.DeepEqual(state, after) {
				t.Fatal("legacy PUT changed submission state")
			}
			// saved=false must not disable category validation.
			before = editSnapshot(t, f)
			requireSubmissionHTTP(t, editRequest(f, `{"category_ids":[]}`), 400)
			if editSnapshot(t, f) != before {
				t.Fatal("invalidated selection lost categories")
			}
		})
	}
}
