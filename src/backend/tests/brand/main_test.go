package brand_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"backend/routers"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testDB *pgxpool.Pool
var testImageRoot string

func TestMain(m *testing.M) {
	os.Exit(runBrandTests(m))
}

func runBrandTests(m *testing.M) (code int) {
	ctx := context.Background()

	host := os.Getenv("TEST_DB_HOST")
	port := os.Getenv("TEST_DB_PORT")
	user := os.Getenv("TEST_DB_USER")
	password := os.Getenv("TEST_DB_PASSWORD")
	dbName := os.Getenv("TEST_DB_NAME")

	if host == "" ||
		port == "" ||
		user == "" ||
		password == "" ||
		dbName == "" {
		panic("test database environment variables are not configured")
	}
	if dbName != "auma_test" {
		panic("brand tests require TEST_DB_NAME=auma_test")
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		dbName,
	)

	var err error

	testDB, err = pgxpool.New(
		ctx,
		databaseURL,
	)
	if err != nil {
		panic(err)
	}

	if err := testDB.Ping(ctx); err != nil {
		testDB.Close()
		panic(err)
	}
	defer testDB.Close()

	var actualDatabase string
	if err := testDB.QueryRow(ctx, "SELECT current_database()").Scan(&actualDatabase); err != nil {
		panic(err)
	}
	if actualDatabase != "auma_test" {
		panic("brand tests must connect only to auma_test")
	}

	// Router生成前に差し替え、Composeの実画像Volumeには保存しない。
	testImageRoot, err = os.MkdirTemp("", "auma-brand-http-images-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(testImageRoot); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup test image root: %v\n", err)
			code = 1
		}
	}()
	if err := os.Setenv("STORAGE_LOCAL_ROOT", testImageRoot); err != nil {
		panic(err)
	}

	// テスト実行時もJSONリクエスト本文を取得できるようにする
	beego.BConfig.CopyRequestBody = true

	routers.Register(
		testDB,
		nil,
	)

	return m.Run()
}
