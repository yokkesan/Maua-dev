package registration_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"backend/internal/registration"
	"backend/tests/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
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
		email           string
		company         string
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

func TestPostgresRepositoryCompletesBrandRegistration(t *testing.T) {
	db := testutil.NewTestPostgresPool(t)

	ctx := context.Background()

	cleanRegistrationTables(t, ctx, db)

	repository := registration.NewPostgresRepository(db)

	tokenHash := sha256.Sum256([]byte("brand-registration-token"))

	var tokenID int64

	err := db.QueryRow(
		ctx,
		`
			INSERT INTO signup_tokens (
				invitation_email,
				token_hash,
				company_name,
				company_phonetic,
				company_post_code,
				company_address,
				company_tel,
				company_type,
				representative_name,
				representative_phonetic,
				registration_plan,
				expires_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12
			)
			RETURNING id
		`,
		"brand-register@example.com",
		tokenHash[:],
		"テストブランド株式会社",
		"テストブランドカブシキガイシャ",
		"100-0001",
		"東京都千代田区",
		"03-1234-5678",
		registration.CompanyTypeBrand,
		"山田太郎",
		"ヤマダタロウ",
		registration.RegistrationPlanBrandFree,
		time.Now().UTC().Add(time.Hour),
	).Scan(&tokenID)
	if err != nil {
		t.Fatalf("insert signup token: %v", err)
	}

	result, err := repository.CompleteRegistration(
		ctx,
		registration.CompleteRegistrationParams{
			TokenID: tokenID,

			Email:        "brand-register@example.com",
			PasswordHash: "$2a$10$test-password-hash",

			CompanyName:            "テストブランド株式会社",
			CompanyPhonetic:        "テストブランドカブシキガイシャ",
			CompanyPostCode:        "100-0001",
			CompanyAddress:         "東京都千代田区",
			CompanyTel:             "03-1234-5678",
			CompanyType:            registration.CompanyTypeBrand,
			RepresentativeName:     "山田太郎",
			RepresentativePhonetic: "ヤマダタロウ",
			RegistrationPlan:       registration.RegistrationPlanBrandFree,
		},
	)
	if err != nil {
		t.Fatalf("CompleteRegistration returned error: %v", err)
	}

	if result.CompanyID == 0 {
		t.Fatal("company id should not be zero")
	}

	if result.UserID == 0 {
		t.Fatal("user id should not be zero")
	}

	if result.BrandID == nil || *result.BrandID == 0 {
		t.Fatal("brand id should not be nil or zero")
	}

	if result.ShopID != nil {
		t.Fatal("shop id must be nil for brand registration")
	}

	var (
		companyName string
		companyType int16
	)

	err = db.QueryRow(
		ctx,
		`
			SELECT
				name,
				company_type
			FROM companies
			WHERE id = $1
		`,
		result.CompanyID,
	).Scan(
		&companyName,
		&companyType,
	)
	if err != nil {
		t.Fatalf("query company: %v", err)
	}

	if companyName != "テストブランド株式会社" {
		t.Fatalf("unexpected company name: %s", companyName)
	}

	if companyType != registration.CompanyTypeBrand {
		t.Fatalf("unexpected company type: %d", companyType)
	}

	var (
		userEmail    string
		passwordHash string
		userType     int16
	)

	err = db.QueryRow(
		ctx,
		`
			SELECT
				email,
				password_hash,
				user_type
			FROM users
			WHERE id = $1
		`,
		result.UserID,
	).Scan(
		&userEmail,
		&passwordHash,
		&userType,
	)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}

	if userEmail != "brand-register@example.com" {
		t.Fatalf("unexpected user email: %s", userEmail)
	}

	if passwordHash != "$2a$10$test-password-hash" {
		t.Fatalf("unexpected password hash: %s", passwordHash)
	}

	if userType != registration.UserTypeBrand {
		t.Fatalf("unexpected user type: %d", userType)
	}

	var (
		brandCompanyID int64
		brandName      string
		brandEmail     string
	)

	err = db.QueryRow(
		ctx,
		`
			SELECT
				company_id,
				name,
				email
			FROM brands
			WHERE id = $1
		`,
		*result.BrandID,
	).Scan(
		&brandCompanyID,
		&brandName,
		&brandEmail,
	)
	if err != nil {
		t.Fatalf("query brand: %v", err)
	}

	if brandCompanyID != result.CompanyID {
		t.Fatalf(
			"expected brand company id %d, got %d",
			result.CompanyID,
			brandCompanyID,
		)
	}

	if brandName != "テストブランド株式会社" {
		t.Fatalf("unexpected brand name: %s", brandName)
	}

	if brandEmail != "brand-register@example.com" {
		t.Fatalf("unexpected brand email: %s", brandEmail)
	}

	var consumedAt *time.Time

	err = db.QueryRow(
		ctx,
		`
			SELECT consumed_at
			FROM signup_tokens
			WHERE id = $1
		`,
		tokenID,
	).Scan(&consumedAt)
	if err != nil {
		t.Fatalf("query consumed_at: %v", err)
	}

	if consumedAt == nil {
		t.Fatal("signup token should be consumed")
	}
}

func TestPostgresRepositoryCompletesShopRegistration(t *testing.T) {
	db := testutil.NewTestPostgresPool(t)

	ctx := context.Background()

	cleanRegistrationTables(t, ctx, db)

	repository := registration.NewPostgresRepository(db)

	tokenHash := sha256.Sum256([]byte("shop-registration-token"))

	var tokenID int64

	err := db.QueryRow(
		ctx,
		`
			INSERT INTO signup_tokens (
				invitation_email,
				token_hash,
				company_name,
				company_type,
				registration_plan,
				expires_at
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6
			)
			RETURNING id
		`,
		"shop-register@example.com",
		tokenHash[:],
		"テストショップ",
		registration.CompanyTypeShop,
		registration.RegistrationPlanShopUnit,
		time.Now().UTC().Add(time.Hour),
	).Scan(&tokenID)
	if err != nil {
		t.Fatalf("insert signup token: %v", err)
	}

	result, err := repository.CompleteRegistration(
		ctx,
		registration.CompleteRegistrationParams{
			TokenID: tokenID,

			Email:        "shop-register@example.com",
			PasswordHash: "$2a$10$test-password-hash",

			CompanyName:      "テストショップ",
			CompanyType:      registration.CompanyTypeShop,
			RegistrationPlan: registration.RegistrationPlanShopUnit,
		},
	)
	if err != nil {
		t.Fatalf("CompleteRegistration returned error: %v", err)
	}

	if result.CompanyID == 0 {
		t.Fatal("company id should not be zero")
	}

	if result.UserID == 0 {
		t.Fatal("user id should not be zero")
	}

	if result.ShopID == nil || *result.ShopID == 0 {
		t.Fatal("shop id should not be nil or zero")
	}

	if result.BrandID != nil {
		t.Fatal("brand id must be nil for shop registration")
	}

	var (
		shopCompanyID    int64
		shopName         string
		shopEmail        string
		registrationPlan int16
	)

	err = db.QueryRow(
		ctx,
		`
			SELECT
				company_id,
				name,
				email,
				registration_plan
			FROM shops
			WHERE id = $1
		`,
		*result.ShopID,
	).Scan(
		&shopCompanyID,
		&shopName,
		&shopEmail,
		&registrationPlan,
	)
	if err != nil {
		t.Fatalf("query shop: %v", err)
	}

	if shopCompanyID != result.CompanyID {
		t.Fatalf(
			"expected shop company id %d, got %d",
			result.CompanyID,
			shopCompanyID,
		)
	}

	if shopName != "テストショップ" {
		t.Fatalf("unexpected shop name: %s", shopName)
	}

	if shopEmail != "shop-register@example.com" {
		t.Fatalf("unexpected shop email: %s", shopEmail)
	}

	if registrationPlan != registration.RegistrationPlanShopUnit {
		t.Fatalf(
			"unexpected registration plan: %d",
			registrationPlan,
		)
	}

	var consumedAt *time.Time

	err = db.QueryRow(
		ctx,
		`
			SELECT consumed_at
			FROM signup_tokens
			WHERE id = $1
		`,
		tokenID,
	).Scan(&consumedAt)
	if err != nil {
		t.Fatalf("query consumed_at: %v", err)
	}

	if consumedAt == nil {
		t.Fatal("signup token should be consumed")
	}
}

func cleanRegistrationTables(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
) {
	t.Helper()

	_, err := db.Exec(
		ctx,
		`
			TRUNCATE TABLE
				brands,
				shops,
				users,
				companies,
				signup_tokens
			RESTART IDENTITY
			CASCADE
		`,
	)
	if err != nil {
		t.Fatalf(
			"clean registration tables: %v",
			err,
		)
	}
}