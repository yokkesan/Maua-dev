package registration_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"backend/internal/registration"
	"backend/tests/testutil"
)

func TestPostgresRepositoryCreatesVerificationToken(t *testing.T) {
	db := testutil.NewTestPostgresPool(t)

	ctx := context.Background()

	_, err := db.Exec(ctx, "DELETE FROM signup_tokens")
	if err != nil {
		t.Fatalf("clean signup_tokens: %v", err)
	}

	repository := registration.NewPostgresRepository(db)

	rawToken := "test-token"
	tokenHash := sha256.Sum256([]byte(rawToken))

	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	err = repository.CreateVerificationToken(
		ctx,
		registration.CreateTokenParams{
			VerificationInput: registration.VerificationInput{
				Email:                  "test@example.com",
				CompanyName:            "テスト株式会社",
				CompanyPhonetic:        "テストカブシキガイシャ",
				CompanyPostCode:        "100-0001",
				CompanyAddress:         "東京都千代田区",
				CompanyTel:             "03-1234-5678",
				CompanyType:            registration.CompanyTypeBrand,
				RepresentativeName:     "山田太郎",
				RepresentativePhonetic: "ヤマダタロウ",
				RegistrationPlan:       registration.RegistrationPlanBrandFree,
			},
			TokenHash: tokenHash[:],
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		t.Fatalf("CreateVerificationToken returned error: %v", err)
	}

	var (
		email     string
		company   string
		tokenHashFromDB []byte
	)

	err = db.QueryRow(
		ctx,
		`
			SELECT
				invitation_email,
				company_name,
				token_hash
			FROM signup_tokens
			WHERE invitation_email = $1
		`,
		"test@example.com",
	).Scan(
		&email,
		&company,
		&tokenHashFromDB,
	)
	if err != nil {
		t.Fatalf("query signup token: %v", err)
	}

	if email != "test@example.com" {
		t.Fatalf("unexpected email: %s", email)
	}

	if company != "テスト株式会社" {
		t.Fatalf("unexpected company name: %s", company)
	}

	if string(tokenHashFromDB) != string(tokenHash[:]) {
		t.Fatal("token hash does not match")
	}
}

func TestPostgresRepositoryRevokesExistingToken(t *testing.T) {
	db := testutil.NewTestPostgresPool(t)

	ctx := context.Background()

	_, err := db.Exec(ctx, "DELETE FROM signup_tokens")
	if err != nil {
		t.Fatalf("clean signup_tokens: %v", err)
	}

	repository := registration.NewPostgresRepository(db)

	firstHash := sha256.Sum256([]byte("first-token"))
	secondHash := sha256.Sum256([]byte("second-token"))

	firstParams := registration.CreateTokenParams{
		VerificationInput: registration.VerificationInput{
			Email:            "same@example.com",
			CompanyName:      "テスト株式会社",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
		},
		TokenHash: firstHash[:],
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}

	err = repository.CreateVerificationToken(ctx, firstParams)
	if err != nil {
		t.Fatalf("create first token: %v", err)
	}

	secondParams := firstParams
	secondParams.TokenHash = secondHash[:]

	err = repository.CreateVerificationToken(ctx, secondParams)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	rows, err := db.Query(
		ctx,
		`
			SELECT token_hash, revoked_at
			FROM signup_tokens
			WHERE invitation_email = $1
			ORDER BY id ASC
		`,
		"same@example.com",
	)
	if err != nil {
		t.Fatalf("query signup tokens: %v", err)
	}
	defer rows.Close()

	type tokenRow struct {
		hash      []byte
		revokedAt *time.Time
	}

	var tokens []tokenRow

	for rows.Next() {
		var row tokenRow

		if err := rows.Scan(
			&row.hash,
			&row.revokedAt,
		); err != nil {
			t.Fatalf("scan signup token: %v", err)
		}

		tokens = append(tokens, row)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate signup tokens: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	if tokens[0].revokedAt == nil {
		t.Fatal("first token should be revoked")
	}

	if tokens[1].revokedAt != nil {
		t.Fatal("second token should remain active")
	}
}