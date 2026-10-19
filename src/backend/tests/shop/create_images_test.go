package shop_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"backend/internal/shop"
	"backend/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errImageTestSave = errors.New("injected second image save failure")
var errImageTestCleanup = errors.New("injected image cleanup failure")
var errImageTestCommit = errors.New("injected lost COMMIT acknowledgement")

// 障害注入は各テスト専用のPoolだけに適用する。DDLや共有データの変更はしない。
type imageCreateTrace struct {
	failInsert, failDetail               bool
	inserts, commits, rollbacks, details atomic.Int32
}

func (tr *imageCreateTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	sql := strings.Join(strings.Fields(d.SQL), " ")
	switch {
	case strings.HasPrefix(sql, "INSERT INTO shop_images"):
		if tr.inserts.Add(1) == 2 && tr.failInsert {
			// 2件目をslot=1にし、実DBのUNIQUE制約違反を発生させる。
			// 1件目のINSERTもRollbackされたことを後で別接続から確認する。
			d.Args[1] = 1
		}
	case strings.HasPrefix(sql, "SELECT") && strings.Contains(sql, "FROM shops"):
		tr.details.Add(1)
		if tr.failDetail {
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			return canceled
		}
	case sql == "commit":
		tr.commits.Add(1)
	case sql == "rollback":
		tr.rollbacks.Add(1)
	}
	return ctx
}

func (*imageCreateTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// PostgreSQLが実際にCOMMITした後のCommandCompleteをクライアントへ渡さず切断する。
// 単にCommit呼び出し前にContextをキャンセルするテストとは区別する。
type lostCommitConn struct {
	net.Conn
	reader  *bufio.Reader
	pending []byte
	dropped *atomic.Bool
}

func (c *lostCommitConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) == 0 {
		header := make([]byte, 5)
		if _, err := io.ReadFull(c.reader, header); err != nil {
			return 0, err
		}
		size := int(binary.BigEndian.Uint32(header[1:]))
		if size < 4 || size > 16<<20 {
			return 0, fmt.Errorf("unexpected PostgreSQL frame size: %d", size)
		}
		payload := make([]byte, size-4)
		if _, err := io.ReadFull(c.reader, payload); err != nil {
			return 0, err
		}
		if header[0] == 'C' && string(payload) == "COMMIT\x00" {
			c.dropped.Store(true)
			_ = c.Conn.Close()
			return 0, errImageTestCommit
		}
		c.pending = append(header, payload...)
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

type imageCreateStorage struct {
	storage.Storage
	failSecond     bool
	failCleanup    bool
	saved, deleted []string
	payloads       [][]byte
}

func (s *imageCreateStorage) Save(ctx context.Context, key string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if err := s.Storage.Save(ctx, key, bytes.NewReader(data)); err != nil {
		return err
	}
	s.saved = append(s.saved, key)
	s.payloads = append(s.payloads, data)
	if s.failSecond && len(s.saved) == 2 {
		// 失敗対象にもファイルが存在する状態で、2枚とも回収されるか検証する。
		return errImageTestSave
	}
	return nil
}

func (s *imageCreateStorage) Delete(ctx context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	if s.failCleanup {
		return errImageTestCleanup
	}
	return s.Storage.Delete(ctx, key)
}

func imageTestFiles(t *testing.T) []*multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i := 0; i < 2; i++ {
		part, err := w.CreateFormFile("images", fmt.Sprintf("%d.png", i+1))
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		img.SetNRGBA(0, 0, color.NRGBA{R: uint8(100 + i), A: 255})
		if err := png.Encode(part, img); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	// 小さな閾値で一時ファイルも使用し、テスト自身の後始末を明示する。
	if err := req.ParseMultipartForm(1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := req.MultipartForm.RemoveAll(); err != nil {
			t.Error(err)
		}
	})
	return req.MultipartForm.File["images"]
}

func imageTestCompany(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(context.Background(),
		"INSERT INTO companies (name, company_type) VALUES ($1, 0) RETURNING id",
		"image-consistency-"+t.Name()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// このテストが作成した会社と、その配下だけをCASCADE削除する。
		if _, err := testDB.Exec(context.Background(), "DELETE FROM companies WHERE id = $1", id); err != nil {
			t.Errorf("cleanup company %d: %v", id, err)
		}
	})
	return id
}

func imageTestPool(t *testing.T, trace *imageCreateTrace, lostCommit bool, dropped *atomic.Bool) *pgxpool.Pool {
	t.Helper()
	cfg := testDB.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	cfg.ConnConfig.Tracer = trace
	if lostCommit {
		if cfg.ConnConfig.TLSConfig != nil {
			t.Fatal("COMMIT response injection requires the test DB's non-TLS connection")
		}
		dial := cfg.ConnConfig.DialFunc
		cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := dial(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &lostCommitConn{Conn: conn, reader: bufio.NewReader(conn), dropped: dropped}, nil
		}
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func imageTestCounts(t *testing.T, companyID int64, wantShops, wantImages int) {
	t.Helper()
	var shops, images int
	err := testDB.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM shops WHERE company_id=$1),
		(SELECT count(*) FROM shop_images i JOIN shops b ON b.id=i.shop_id WHERE b.company_id=$1)`, companyID).Scan(&shops, &images)
	if err != nil {
		t.Fatal(err)
	}
	if shops != wantShops || images != wantImages {
		t.Fatalf("DB rows: shops=%d images=%d, want %d/%d", shops, images, wantShops, wantImages)
	}
}

func TestShopCreateImageConsistency(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		failSave, failInsert, failDetail, lostCommit bool
	}{
		{name: "success"},
		{name: "second_save_failure", failSave: true},
		{name: "second_image_insert_failure", failInsert: true},
		{name: "detail_failure_after_commit", failDetail: true},
		{name: "lost_commit_acknowledgement", lostCommit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			categoryID := imageTestCategory(t)
			companyID := imageTestCompany(t)
			root := t.TempDir()
			// Cleanupが無関係なファイルを削除しないことも確認する。
			if err := os.WriteFile(filepath.Join(root, "unrelated.txt"), []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			local, err := storage.NewLocalStorage(root)
			if err != nil {
				t.Fatal(err)
			}
			store := &imageCreateStorage{Storage: local, failSecond: tc.failSave}
			images, err := storage.NewMainImageService(store)
			if err != nil {
				t.Fatal(err)
			}
			trace := &imageCreateTrace{failInsert: tc.failInsert, failDetail: tc.failDetail}
			var dropped atomic.Bool
			pool := imageTestPool(t, trace, tc.lostCommit, &dropped)
			svc := shop.NewService(shop.NewPostgresRepository(pool))
			result, err := svc.CreateWithImages(context.Background(), companyID,
				shop.CreateShopInput{"name": json.RawMessage(`"画像テスト"`), "category_ids": json.RawMessage(fmt.Sprintf("[%d]", categoryID)), "registration_plan": json.RawMessage(`10`)}, imageTestFiles(t), images)
			rollback := tc.failSave || tc.failInsert
			var relationCount int
			if queryErr := testDB.QueryRow(context.Background(), "SELECT count(*) FROM shop_categories WHERE category_id=$1", categoryID).Scan(&relationCount); queryErr != nil {
				t.Fatal(queryErr)
			}
			wantRelations := 1
			if rollback {
				wantRelations = 0
			}
			if relationCount != wantRelations {
				t.Fatalf("related rows=%d, want %d", relationCount, wantRelations)
			}
			wantError := rollback || tc.failDetail || tc.lostCommit
			if (err != nil) != wantError {
				t.Fatalf("unexpected result=%v error=%v", result, err)
			}
			if wantError && result != nil {
				t.Fatalf("error returned with result: %v", result)
			}
			if tc.failSave && !errors.Is(err, errImageTestSave) {
				t.Fatalf("wrong save error: %v", err)
			}
			if tc.failInsert {
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
					t.Fatalf("expected actual UNIQUE violation: %v", err)
				}
			}
			if tc.failDetail && !errors.Is(err, context.Canceled) {
				t.Fatalf("wrong detail error: %v", err)
			}
			if tc.lostCommit && (!dropped.Load() || !strings.Contains(err.Error(), "commit shop create")) {
				t.Fatalf("COMMIT response loss was not exercised: dropped=%v err=%v", dropped.Load(), err)
			}
			if len(store.saved) != 2 {
				t.Fatalf("saved files=%d, want 2", len(store.saved))
			}
			wantCommits, wantRollbacks, wantDetails, wantInserts := int32(1), int32(0), int32(1), int32(2)
			if rollback {
				wantCommits, wantRollbacks, wantDetails = 0, 1, 0
			}
			if tc.lostCommit {
				wantDetails = 0
			}
			if tc.failSave {
				wantInserts = 0
			}
			if trace.commits.Load() != wantCommits || trace.rollbacks.Load() != wantRollbacks || trace.details.Load() != wantDetails || trace.inserts.Load() != wantInserts {
				t.Fatalf("unexpected SQL path: commit=%d rollback=%d detail=%d image inserts=%d", trace.commits.Load(), trace.rollbacks.Load(), trace.details.Load(), trace.inserts.Load())
			}
			if rollback {
				imageTestCounts(t, companyID, 0, 0)
				if !reflect.DeepEqual(store.deleted, store.saved) {
					t.Fatalf("cleanup keys=%v, want %v", store.deleted, store.saved)
				}
				for _, key := range store.saved {
					if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(key))); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("file not removed: %s: %v", key, err)
					}
				}
			} else {
				// 応答を失ったケースでも、独立した通常接続から確定済みDBを確認する。
				imageTestCounts(t, companyID, 1, 2)
				if len(store.deleted) != 0 {
					t.Fatalf("committed images were deleted: %v", store.deleted)
				}
				rows, err := testDB.Query(context.Background(), `SELECT i.shop_id, i.slot_number, i.storage_key
					FROM shop_images i JOIN shops b ON b.id=i.shop_id WHERE b.company_id=$1 ORDER BY i.slot_number`, companyID)
				if err != nil {
					t.Fatal(err)
				}
				defer rows.Close()
				i := 0
				for rows.Next() {
					var id int64
					var slot int
					var key string
					if err := rows.Scan(&id, &slot, &key); err != nil {
						t.Fatal(err)
					}
					if i >= 2 || slot != i+1 || key != store.saved[i] || !strings.HasPrefix(key, fmt.Sprintf("shops/%d/main/%d_", id, slot)) {
						t.Fatalf("unexpected image row: id=%d slot=%d key=%s", id, slot, key)
					}
					if !wantError && result["id"] != id {
						t.Fatalf("response ID=%v, persisted ID=%d", result["id"], id)
					}
					data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(key)))
					if err != nil || !bytes.Equal(data, store.payloads[i]) {
						t.Fatalf("saved image mismatch: %s: %v", key, err)
					}
					i++
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				if i != 2 {
					t.Fatalf("image rows=%d, want 2", i)
				}
			}
			if data, err := os.ReadFile(filepath.Join(root, "unrelated.txt")); err != nil || string(data) != "keep" {
				t.Fatalf("unrelated file changed: %v", err)
			}
		})
	}
}

func TestShopCreateJSONWithoutImagesRegression(t *testing.T) {
	companyID := imageTestCompany(t)
	svc := shop.NewService(shop.NewPostgresRepository(testDB))
	for i := 0; i < 2; i++ {
		var input shop.CreateShopInput
		if err := json.Unmarshal([]byte(`{"name":"JSON登録","registration_plan":10,"concept":"既存登録","net_shop":1,"category_ids":[]}`), &input); err != nil {
			t.Fatal(err)
		}
		result, err := svc.Create(context.Background(), companyID, input)
		if err != nil {
			t.Fatal(err)
		}
		if result["name"] != "JSON登録" || result["concept"] != "既存登録" || result["net_shop"] != int16(1) || result["company_id"] != companyID {
			t.Fatalf("unexpected JSON registration result: %v", result)
		}
		if ids, ok := result["category_ids"].([]int64); !ok || ids == nil || len(ids) != 0 {
			t.Fatalf("unexpected relation: %v", result["category_ids"])
		}
		imageTestCounts(t, companyID, i+1, 0)
	}
	for _, body := range []string{`{}`, `{"name":" "}`, `{"name":"x","company_id":1}`, `{"name":"x","category_ids":[1,1]}`} {
		var input shop.CreateShopInput
		if err := json.Unmarshal([]byte(body), &input); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Create(context.Background(), companyID, input); !errors.Is(err, shop.ErrInvalidShopInput) {
			t.Fatalf("input %s: expected validation failure, got %v", body, err)
		}
	}
	imageTestCounts(t, companyID, 2, 0)
}

func imageTestCategory(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := testDB.QueryRow(context.Background(), `INSERT INTO categories (first_id,second_id)
 SELECT first_category_id,id FROM second_categories ORDER BY id LIMIT 1 RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := testDB.Exec(context.Background(), "DELETE FROM categories WHERE id=$1", id); err != nil {
			t.Errorf("cleanup category %d: %v", id, err)
		}
	})
	return id
}

func TestShopImageCleanupFailureIsReturned(t *testing.T) {
	for _, insertFailure := range []bool{false, true} {
		t.Run(fmt.Sprintf("insert_failure_%v", insertFailure), func(t *testing.T) {
			companyID := imageTestCompany(t)
			root := t.TempDir()
			local, err := storage.NewLocalStorage(root)
			if err != nil {
				t.Fatal(err)
			}
			store := &imageCreateStorage{Storage: local, failSecond: !insertFailure, failCleanup: true}
			images, err := storage.NewMainImageService(store)
			if err != nil {
				t.Fatal(err)
			}
			trace := &imageCreateTrace{failInsert: insertFailure}
			var dropped atomic.Bool
			svc := shop.NewService(shop.NewPostgresRepository(imageTestPool(t, trace, false, &dropped)))
			_, err = svc.CreateWithImages(context.Background(), companyID, shop.CreateShopInput{"name": json.RawMessage(`"cleanup失敗"`), "registration_plan": json.RawMessage(`10`)}, imageTestFiles(t), images)
			if !errors.Is(err, errImageTestCleanup) {
				t.Fatalf("cleanup failure lost: %v", err)
			}
			if insertFailure {
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "23505" {
					t.Fatalf("original insert failure lost: %v", err)
				}
			} else if !errors.Is(err, errImageTestSave) {
				t.Fatalf("original save failure lost: %v", err)
			}
			imageTestCounts(t, companyID, 0, 0)
			if trace.rollbacks.Load() != 1 || trace.commits.Load() != 0 {
				t.Fatal("expected rollback without commit")
			}
			if len(store.saved) != 2 || !reflect.DeepEqual(store.deleted, store.saved) {
				t.Fatalf("cleanup not attempted for both files: %v", store.deleted)
			}
			// 削除失敗時にはファイルが残り得る。テスト専用ディレクトリはt.TempDirで回収する。
			for _, key := range store.saved {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(key))); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
