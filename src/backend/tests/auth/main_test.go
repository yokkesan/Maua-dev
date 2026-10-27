package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"backend/routers"

	"github.com/jackc/pgx/v5/pgxpool"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	host := os.Getenv("TEST_DB_HOST")
	port := os.Getenv("TEST_DB_PORT")
	name := os.Getenv("TEST_DB_NAME")
	user := os.Getenv("TEST_DB_USER")
	password := os.Getenv("TEST_DB_PASSWORD")

	if host == "" ||
		port == "" ||
		name == "" ||
		user == "" ||
		password == "" {
		panic("test database environment variables are not configured")
	}

	if name != "auma_test" {
		panic("auth tests require TEST_DB_NAME=auma_test")
	}

	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user,
		password,
		host,
		port,
		name,
	)

	var err error

	testDB, err = pgxpool.New(ctx, databaseURL)
	if err != nil {
		panic(err)
	}

	if err := testDB.Ping(ctx); err != nil {
		testDB.Close()
		panic(err)
	}

	var actualDatabase string
	if err := testDB.QueryRow(ctx, "SELECT current_database()").Scan(&actualDatabase); err != nil || actualDatabase != "auma_test" {
		testDB.Close()
		panic("auth tests must connect only to auma_test")
	}

	routers.Register(
		testDB,
		nil,
	)

	code := m.Run()

	testDB.Close()

	os.Exit(code)
}
