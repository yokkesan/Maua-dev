package brand_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const testPassword = "Test1234!"

type testUser struct {
	ID        int64
	CompanyID int64
	Email     string
	UserType  int16
	Role      int16
}

func createTestUser(
	t *testing.T,
	userType int16,
) testUser {
	t.Helper()

	ctx := context.Background()

	companyType := int16(0)
	if userType == 2 {
		companyType = 1
	}

	testName := strings.NewReplacer(
		"/", "-",
		" ", "-",
	).Replace(t.Name())

	email := fmt.Sprintf(
		"%s-%d@example.com",
		strings.ToLower(testName),
		time.Now().UnixNano(),
	)

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(testPassword),
		bcrypt.DefaultCost,
	)
	if err != nil {
		t.Fatalf(
			"failed to hash password: %v",
			err,
		)
	}

	var companyID int64

	err = testDB.QueryRow(
		ctx,
		`
			INSERT INTO companies (
				name,
				company_type
			)
			VALUES ($1, $2)
			RETURNING id
		`,
		"Brand APIテスト株式会社",
		companyType,
	).Scan(&companyID)
	if err != nil {
		t.Fatalf(
			"failed to create company: %v",
			err,
		)
	}

	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			`
				DELETE FROM companies
				WHERE id = $1
			`,
			companyID,
		)
	})

	var userID int64

	err = testDB.QueryRow(
		ctx,
		`
			INSERT INTO users (
				company_id,
				email,
				password_hash,
				user_type,
				role
			)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`,
		companyID,
		email,
		string(passwordHash),
		userType,
		int16(0),
	).Scan(&userID)
	if err != nil {
		t.Fatalf(
			"failed to create user: %v",
			err,
		)
	}

	if userType == 2 {
		_, err = testDB.Exec(
			ctx,
			`
				INSERT INTO brands (
					company_id,
					name,
					phonetic,
					tel,
					email
				)
				VALUES (
					$1,
					$2,
					$3,
					$4,
					$5
				)
			`,
			companyID,
			"Brand APIテストブランド",
			"ブランドエーピーアイテストブランド",
			"097-000-0000",
			email,
		)
		if err != nil {
			t.Fatalf(
				"failed to create brand: %v",
				err,
			)
		}
	}

	return testUser{
		ID:        userID,
		CompanyID: companyID,
		Email:     email,
		UserType:  userType,
		Role:      0,
	}
}
