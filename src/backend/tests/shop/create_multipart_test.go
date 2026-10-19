package shop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"backend/internal/storage"

	beego "github.com/beego/beego/v2/server/web"
)

type multipartShopImage struct {
	name, contentType string
	data              []byte
}

func makeMultipartShopImage(t *testing.T, format string) multipartShopImage {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	img.SetNRGBA(0, 0, color.NRGBA{R: 180, G: 80, A: 255})
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg":
		err = jpeg.Encode(&buf, img, nil)
	case "gif":
		err = gif.Encode(&buf, img, nil)
	default:
		t.Fatalf("unsupported fixture format: %s", format)
	}
	if err != nil {
		t.Fatal(err)
	}
	return multipartShopImage{name: "main." + format, contentType: "image/" + format, data: buf.Bytes()}
}

func postMultipartShop(t *testing.T, cookie *http.Cookie, data []string, images []multipartShopImage, unknownLength ...bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, value := range data {
		if err := w.WriteField("data", value); err != nil {
			t.Fatal(err)
		}
	}
	for _, img := range images {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="main_images"; filename="%s"`, img.name))
		header.Set("Content-Type", img.contentType)
		part, err := w.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(img.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shops", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if len(unknownLength) > 0 && unknownLength[0] {
		req.ContentLength = -1
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	// BeforeExecの認証で終了した場合も、パーサーが作った一時ファイルを回収する。
	t.Cleanup(func() {
		if req.MultipartForm != nil {
			if err := req.MultipartForm.RemoveAll(); err != nil {
				t.Error(err)
			}
		}
	})
	rec := httptest.NewRecorder()
	beego.BeeApp.Handlers.ServeHTTP(rec, req)
	return rec
}

func multipartImageFiles(t *testing.T) []string {
	t.Helper()
	files := []string{}
	if err := filepath.WalkDir(testImageRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func cleanupMultipartShopFiles(t *testing.T, companyID int64) {
	t.Helper()
	t.Cleanup(func() {
		// createTestUserの会社Cleanupより先に実行する。テスト自身の会社配下だけが対象。
		rows, err := testDB.Query(context.Background(), "SELECT id FROM shops WHERE company_id=$1", companyID)
		if err != nil {
			t.Errorf("find test shops for image cleanup: %v", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Error(err)
				return
			}
			if err := os.RemoveAll(filepath.Join(testImageRoot, "shops", fmt.Sprint(id))); err != nil {
				t.Error(err)
			}
		}
		if err := rows.Err(); err != nil {
			t.Error(err)
		}
	})
}

func TestCreateShopMultipartSuccess(t *testing.T) {
	for _, formats := range [][]string{{"jpeg", "png"}, {"png", "png"}, {"jpeg", "jpeg"}, {"png", "jpeg"}} {
		t.Run(strings.Join(formats, "_"), func(t *testing.T) {
			user := createTestUser(t, 1)
			cleanupMultipartShopFiles(t, user.CompanyID)
			cookie := loginForShopCreateTest(t, user.Email)
			before := countShopsForCreateTest(t, user.CompanyID)
			filesBefore := multipartImageFiles(t)
			images := []multipartShopImage{makeMultipartShopImage(t, formats[0]), makeMultipartShopImage(t, formats[1])}
			if formats[0] == "png" && formats[1] == "jpeg" {
				images[0].name, images[0].contentType = "../../outside.php", "text/plain"
				images[1].name, images[1].contentType = "fake.svg", "image/svg+xml"
			}
			plan := int64(10)
			if formats[0] == "png" && formats[1] == "jpeg" {
				plan = 11
			}
			rec := postMultipartShop(t, cookie, []string{fmt.Sprintf(`{"registration_plan":%d,"name":"画像付きHTTP登録","concept":"multipart結合テスト","category_ids":[]}`, plan)}, images)
			if rec.Code != http.StatusCreated {
				t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Shop struct {
					RegistrationPlan int64  `json:"registration_plan"`
					ID               int64  `json:"id"`
					CompanyID        int64  `json:"company_id"`
					Name             string `json:"name"`
					Concept          string `json:"concept"`
				} `json:"shop"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Shop.RegistrationPlan != plan || body.Shop.ID <= 0 || body.Shop.CompanyID != user.CompanyID || body.Shop.Name != "画像付きHTTP登録" || body.Shop.Concept != "multipart結合テスト" {
				t.Fatalf("unexpected response: %s", rec.Body.String())
			}
			imageTestCounts(t, user.CompanyID, before+1, 2)
			var name, concept string
			var savedPlan int64
			if err := testDB.QueryRow(context.Background(), "SELECT name, concept, registration_plan FROM shops WHERE id=$1 AND company_id=$2", body.Shop.ID, user.CompanyID).Scan(&name, &concept, &savedPlan); err != nil {
				t.Fatal(err)
			}
			if savedPlan != plan || name != body.Shop.Name || concept != body.Shop.Concept {
				t.Fatalf("shop not persisted correctly: name=%q concept=%q", name, concept)
			}
			rows, err := testDB.Query(context.Background(), "SELECT slot_number, storage_key FROM shop_images WHERE shop_id=$1 ORDER BY slot_number", body.Shop.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			i := 0
			for rows.Next() {
				var slot int
				var key string
				if err := rows.Scan(&slot, &key); err != nil {
					t.Fatal(err)
				}
				if i >= 2 || slot != i+1 || !strings.HasPrefix(key, fmt.Sprintf("shops/%d/main/%d_", body.Shop.ID, slot)) {
					t.Fatalf("unexpected image: slot=%d key=%q", slot, key)
				}
				expectedExt := ".png"
				if formats[i] == "jpeg" {
					expectedExt = ".jpg"
				}
				if filepath.Ext(key) != expectedExt {
					t.Fatalf("untrusted filename affected storage key: %q", key)
				}
				path := filepath.Join(testImageRoot, filepath.FromSlash(key))
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("image missing for storage_key=%q: %v", key, err)
				}
				if !bytes.Equal(data, images[i].data) {
					t.Fatalf("image bytes differ at slot %d", slot)
				}
				if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
					t.Fatalf("saved image is invalid: %v", err)
				}
				i++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if i != 2 {
				t.Fatalf("image rows=%d, want 2", i)
			}
			if got := len(multipartImageFiles(t)); got != len(filesBefore)+2 {
				t.Fatalf("stored file count=%d, want %d", got, len(filesBefore)+2)
			}
		})
	}
}

func TestCreateShopMultipartRejected(t *testing.T) {
	pngImage := makeMultipartShopImage(t, "png")
	jpegImage := makeMultipartShopImage(t, "jpeg")
	gifImage := makeMultipartShopImage(t, "gif")
	// JPEG/PNGと偽ったファイル名・MIMEでも、実データで拒否されることを確認する。
	gifImage.name, gifImage.contentType = "fake.png", "image/png"
	brokenPNG := pngImage
	brokenPNG.data = pngImage.data[:len(pngImage.data)/2]
	brokenJPEG := jpegImage
	brokenJPEG.data = jpegImage.data[:len(jpegImage.data)/2]
	oversized := pngImage
	oversized.data = make([]byte, storage.MaxImageSize+1)
	copy(oversized.data, pngImage.data)
	validData := []string{`{"registration_plan":10,"name":"拒否対象ショップ"}`}
	for _, tc := range []struct {
		name     string
		images   []multipartShopImage
		data     []string
		userType int16
		noAuth   bool
		status   int
	}{
		{name: "company_id_forbidden", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`{"registration_plan":10,"name":"拒否対象ショップ","company_id":1}`}, status: 400},
		{name: "missing_plan", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`{"name":"x"}`}, status: 400},
		{name: "invalid_plan", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`{"name":"x","registration_plan":0}`}, status: 400},
		{name: "one_image", images: []multipartShopImage{pngImage}, data: validData, status: 400},
		{name: "three_images", images: []multipartShopImage{pngImage, jpegImage, pngImage}, data: validData, status: 400},
		{name: "unsupported_gif", images: []multipartShopImage{pngImage, gifImage}, data: validData, status: 400},
		{name: "invalid_bytes", images: []multipartShopImage{pngImage, {name: "fake.jpg", contentType: "image/jpeg", data: []byte("not an image")}}, data: validData, status: 400},
		{name: "corrupt_png", images: []multipartShopImage{pngImage, brokenPNG}, data: validData, status: 400},
		{name: "corrupt_jpeg", images: []multipartShopImage{pngImage, brokenJPEG}, data: validData, status: 400},
		{name: "over_5MB", images: []multipartShopImage{pngImage, oversized}, data: validData, status: 400},
		{name: "unauthenticated", images: []multipartShopImage{pngImage, jpegImage}, data: validData, noAuth: true, status: 401},
		{name: "brand_user", images: []multipartShopImage{pngImage, jpegImage}, data: validData, userType: 2, status: 403},
		{name: "missing_data", images: []multipartShopImage{pngImage, jpegImage}, status: 400},
		{name: "duplicate_data", images: []multipartShopImage{pngImage, jpegImage}, data: []string{validData[0], validData[0]}, status: 400},
		{name: "empty_data", images: []multipartShopImage{pngImage, jpegImage}, data: []string{""}, status: 400},
		{name: "malformed_json", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`{"registration_plan":10,"name":`}, status: 400},
		{name: "json_array", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`[]`}, status: 400},
		{name: "extra_json_value", images: []multipartShopImage{pngImage, jpegImage}, data: []string{validData[0] + ` {}`}, status: 400},
		{name: "invalid_shop_input", images: []multipartShopImage{pngImage, jpegImage}, data: []string{`{"registration_plan":10,"name":" "}`}, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			userType := tc.userType
			if userType == 0 {
				userType = 1
			}
			user := createTestUser(t, userType)
			cleanupMultipartShopFiles(t, user.CompanyID)
			var cookie *http.Cookie
			if !tc.noAuth {
				cookie = loginForShopCreateTest(t, user.Email)
			}
			before := countShopsForCreateTest(t, user.CompanyID)
			filesBefore := multipartImageFiles(t)
			rec := postMultipartShop(t, cookie, tc.data, tc.images)
			if rec.Code != tc.status {
				t.Fatalf("want %d, got %d: %s", tc.status, rec.Code, rec.Body.String())
			}
			imageTestCounts(t, user.CompanyID, before, 0)
			if filesAfter := multipartImageFiles(t); !reflect.DeepEqual(filesBefore, filesAfter) {
				t.Fatalf("rejected request changed files: before=%v after=%v", filesBefore, filesAfter)
			}
		})
	}
}

func TestCreateShopJSONHTTPRegression(t *testing.T) {
	user := createTestUser(t, 1)
	cleanupMultipartShopFiles(t, user.CompanyID)
	cookie := loginForShopCreateTest(t, user.Email)
	before := countShopsForCreateTest(t, user.CompanyID)
	filesBefore := multipartImageFiles(t)
	rec := postShopForTest(t, cookie, `{"registration_plan":10,"name":"従来のJSON登録","concept":"画像なし","category_ids":[]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Shop struct {
			ID        int64  `json:"id"`
			CompanyID int64  `json:"company_id"`
			Name      string `json:"name"`
		} `json:"shop"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Shop.ID <= 0 || body.Shop.CompanyID != user.CompanyID || body.Shop.Name != "従来のJSON登録" {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	imageTestCounts(t, user.CompanyID, before+1, 0)
	var name string
	if err := testDB.QueryRow(context.Background(), "SELECT name FROM shops WHERE id=$1 AND company_id=$2", body.Shop.ID, user.CompanyID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != body.Shop.Name {
		t.Fatalf("unexpected persisted name: %q", name)
	}
	if filesAfter := multipartImageFiles(t); !reflect.DeepEqual(filesBefore, filesAfter) {
		t.Fatalf("JSON registration changed image files: %v", filesAfter)
	}
}

func TestCreateShopMultipartRequestLimit(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprintf("unknown_length_%v", unknown), func(t *testing.T) {
			user := createTestUser(t, 1)
			cleanupMultipartShopFiles(t, user.CompanyID)
			cookie := loginForShopCreateTest(t, user.Email)
			before := countShopsForCreateTest(t, user.CompanyID)
			filesBefore := multipartImageFiles(t)
			data := `{"name":"` + strings.Repeat("x", 12<<20) + `"}`
			images := []multipartShopImage{makeMultipartShopImage(t, "png"), makeMultipartShopImage(t, "jpeg")}
			rec := postMultipartShop(t, cookie, []string{data}, images, unknown)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("want 413, got %d: %s", rec.Code, rec.Body.String())
			}
			imageTestCounts(t, user.CompanyID, before, 0)
			if filesAfter := multipartImageFiles(t); !reflect.DeepEqual(filesBefore, filesAfter) {
				t.Fatalf("oversized request changed files: %v", filesAfter)
			}
		})
	}
}
