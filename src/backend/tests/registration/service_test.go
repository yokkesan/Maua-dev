package registration_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"backend/internal/registration"

	"golang.org/x/crypto/bcrypt"
)

type fakeRepository struct {
	params registration.CreateTokenParams
	called bool

	findToken  *registration.VerificationToken
	findErr    error
	findCalled bool
	findHash   []byte

	completeParams registration.CompleteRegistrationParams
	completeResult *registration.CompleteRegistrationResult
	completeErr    error
	completeCalled bool
}

func (r *fakeRepository) CreateVerificationToken(
	ctx context.Context,
	params registration.CreateTokenParams,
) error {
	r.params = params
	r.called = true

	return nil
}

func (r *fakeRepository) FindVerificationTokenByHash(
	ctx context.Context,
	tokenHash []byte,
) (*registration.VerificationToken, error) {
	r.findCalled = true
	r.findHash = tokenHash

	return r.findToken, r.findErr
}

func (r *fakeRepository) CompleteRegistration(
	ctx context.Context,
	params registration.CompleteRegistrationParams,
) (*registration.CompleteRegistrationResult, error) {
	r.completeCalled = true
	r.completeParams = params

	return r.completeResult, r.completeErr
}

type fakeMailer struct {
	email  string
	token  string
	called bool
	err    error
}

func (m *fakeMailer) SendVerificationEmail(
	ctx context.Context,
	email string,
	token string,
) error {
	m.email = email
	m.token = token
	m.called = true

	return m.err
}

func TestCreateVerificationBrand(t *testing.T) {
	repository := &fakeRepository{}
	mailer := &fakeMailer{}

	service := registration.NewService(
		repository,
		mailer,
	)

	result, err := service.CreateVerification(
		context.Background(),
		registration.VerificationInput{
			Email:            "test@example.com",
			CompanyName:      "テスト株式会社",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repository.called {
		t.Fatal("repository was not called")
	}

	if result.Token == "" {
		t.Fatal("token should not be empty")
	}

	if len(repository.params.TokenHash) != 32 {
		t.Fatalf(
			"unexpected token hash length: %d",
			len(repository.params.TokenHash),
		)
	}

	if bytes.Equal(
		repository.params.TokenHash,
		[]byte(result.Token),
	) {
		t.Fatal("raw token must not be stored")
	}

	if !mailer.called {
		t.Fatal("mailer was not called")
	}

	if mailer.email != "test@example.com" {
		t.Fatalf(
			"unexpected mail recipient: %s",
			mailer.email,
		)
	}

	if mailer.token != result.Token {
		t.Fatal("mailer must receive the generated raw token")
	}
}

func TestCreateVerificationRejectsInvalidEmail(t *testing.T) {
	repository := &fakeRepository{}
	mailer := &fakeMailer{}

	service := registration.NewService(
		repository,
		mailer,
	)

	_, err := service.CreateVerification(
		context.Background(),
		registration.VerificationInput{
			Email:            "invalid-email",
			CompanyName:      "テスト株式会社",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
		},
	)

	if !errors.Is(err, registration.ErrInvalidEmail) {
		t.Fatalf(
			"expected ErrInvalidEmail, got %v",
			err,
		)
	}

	if repository.called {
		t.Fatal("repository must not be called")
	}

	if mailer.called {
		t.Fatal("mailer must not be called")
	}
}

func TestCreateVerificationRejectsInvalidPlan(t *testing.T) {
	repository := &fakeRepository{}
	mailer := &fakeMailer{}

	service := registration.NewService(
		repository,
		mailer,
	)

	_, err := service.CreateVerification(
		context.Background(),
		registration.VerificationInput{
			Email:            "test@example.com",
			CompanyName:      "テスト株式会社",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanShopPersonal,
		},
	)

	if !errors.Is(err, registration.ErrInvalidPlan) {
		t.Fatalf(
			"expected ErrInvalidPlan, got %v",
			err,
		)
	}

	if repository.called {
		t.Fatal("repository must not be called")
	}

	if mailer.called {
		t.Fatal("mailer must not be called")
	}
}

func TestCreateVerificationReturnsErrorWhenMailFails(t *testing.T) {
	repository := &fakeRepository{}
	mailer := &fakeMailer{
		err: errors.New("mail send failed"),
	}

	service := registration.NewService(
		repository,
		mailer,
	)

	_, err := service.CreateVerification(
		context.Background(),
		registration.VerificationInput{
			Email:            "test@example.com",
			CompanyName:      "テスト株式会社",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if !repository.called {
		t.Fatal("repository should be called before mailer")
	}

	if !mailer.called {
		t.Fatal("mailer was not called")
	}
}

func TestVerifyTokenReturnsValidToken(t *testing.T) {
	expected := &registration.VerificationToken{
		ID:               1,
		InvitationEmail:  "test@example.com",
		CompanyName:      "テスト株式会社",
		CompanyType:      registration.CompanyTypeBrand,
		RegistrationPlan: registration.RegistrationPlanBrandFree,
		ExpiresAt:        time.Now().UTC().Add(time.Hour),
	}

	repository := &fakeRepository{
		findToken: expected,
	}

	service := registration.NewService(
		repository,
		nil,
	)

	result, err := service.VerifyToken(
		context.Background(),
		"valid-token",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repository.findCalled {
		t.Fatal("repository was not called")
	}

	if len(repository.findHash) != 32 {
		t.Fatalf(
			"unexpected token hash length: %d",
			len(repository.findHash),
		)
	}

	if result.ID != expected.ID {
		t.Fatalf(
			"expected token id %d, got %d",
			expected.ID,
			result.ID,
		)
	}
}

func TestVerifyTokenRejectsEmptyToken(t *testing.T) {
	repository := &fakeRepository{}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.VerifyToken(
		context.Background(),
		"",
	)

	if !errors.Is(err, registration.ErrVerificationTokenInvalid) {
		t.Fatalf(
			"expected ErrVerificationTokenInvalid, got %v",
			err,
		)
	}

	if repository.findCalled {
		t.Fatal("repository must not be called")
	}
}

func TestVerifyTokenRejectsUnknownToken(t *testing.T) {
	repository := &fakeRepository{
		findErr: registration.ErrVerificationTokenNotFound,
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.VerifyToken(
		context.Background(),
		"unknown-token",
	)

	if !errors.Is(err, registration.ErrVerificationTokenInvalid) {
		t.Fatalf(
			"expected ErrVerificationTokenInvalid, got %v",
			err,
		)
	}
}

func TestVerifyTokenRejectsExpiredToken(t *testing.T) {
	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:        1,
			ExpiresAt: time.Now().UTC().Add(-time.Hour),
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.VerifyToken(
		context.Background(),
		"expired-token",
	)

	if !errors.Is(err, registration.ErrVerificationTokenExpired) {
		t.Fatalf(
			"expected ErrVerificationTokenExpired, got %v",
			err,
		)
	}
}

func TestVerifyTokenRejectsRevokedToken(t *testing.T) {
	revokedAt := time.Now().UTC()

	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:        1,
			ExpiresAt: time.Now().UTC().Add(time.Hour),
			RevokedAt: &revokedAt,
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.VerifyToken(
		context.Background(),
		"revoked-token",
	)

	if !errors.Is(err, registration.ErrVerificationTokenRevoked) {
		t.Fatalf(
			"expected ErrVerificationTokenRevoked, got %v",
			err,
		)
	}
}

func TestVerifyTokenRejectsConsumedToken(t *testing.T) {
	consumedAt := time.Now().UTC()

	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:         1,
			ExpiresAt:  time.Now().UTC().Add(time.Hour),
			ConsumedAt: &consumedAt,
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.VerifyToken(
		context.Background(),
		"consumed-token",
	)

	if !errors.Is(err, registration.ErrVerificationTokenConsumed) {
		t.Fatalf(
			"expected ErrVerificationTokenConsumed, got %v",
			err,
		)
	}
}

func TestCompleteRegistrationBrand(t *testing.T) {
	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:                     10,
			InvitationEmail:        "brand@example.com",
			CompanyName:            "テストブランド株式会社",
			CompanyPhonetic:        "テストブランドカブシキガイシャ",
			CompanyPostCode:        "100-0001",
			CompanyAddress:         "東京都千代田区",
			CompanyTel:             "03-1234-5678",
			CompanyType:            registration.CompanyTypeBrand,
			RepresentativeName:     "山田太郎",
			RepresentativePhonetic: "ヤマダタロウ",
			RegistrationPlan:       registration.RegistrationPlanBrandFree,
			ExpiresAt:              time.Now().UTC().Add(time.Hour),
		},
		completeResult: &registration.CompleteRegistrationResult{
			CompanyID: 1,
			UserID:    2,
			BrandID:   int64Pointer(3),
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	result, err := service.CompleteRegistration(
		context.Background(),
		"valid-registration-token",
		"password123",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repository.findCalled {
		t.Fatal("verification token repository was not called")
	}

	if !repository.completeCalled {
		t.Fatal("complete registration repository was not called")
	}

	if repository.completeParams.TokenID != 10 {
		t.Fatalf(
			"expected token id 10, got %d",
			repository.completeParams.TokenID,
		)
	}

	if repository.completeParams.Email != "brand@example.com" {
		t.Fatalf(
			"unexpected email: %s",
			repository.completeParams.Email,
		)
	}

	if repository.completeParams.CompanyName != "テストブランド株式会社" {
		t.Fatalf(
			"unexpected company name: %s",
			repository.completeParams.CompanyName,
		)
	}

	if repository.completeParams.CompanyType != registration.CompanyTypeBrand {
		t.Fatalf(
			"unexpected company type: %d",
			repository.completeParams.CompanyType,
		)
	}

	if repository.completeParams.PasswordHash == "" {
		t.Fatal("password hash should not be empty")
	}

	if repository.completeParams.PasswordHash == "password123" {
		t.Fatal("raw password must not be stored")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(repository.completeParams.PasswordHash),
		[]byte("password123"),
	); err != nil {
		t.Fatalf(
			"password hash does not match password: %v",
			err,
		)
	}

	if result.CompanyID != 1 {
		t.Fatalf(
			"expected company id 1, got %d",
			result.CompanyID,
		)
	}

	if result.UserID != 2 {
		t.Fatalf(
			"expected user id 2, got %d",
			result.UserID,
		)
	}

	if result.BrandID == nil || *result.BrandID != 3 {
		t.Fatal("unexpected brand id")
	}

	if result.ShopID != nil {
		t.Fatal("shop id must be nil for brand registration")
	}
}

func TestCompleteRegistrationRejectsEmptyPassword(t *testing.T) {
	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:               1,
			InvitationEmail:  "test@example.com",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
			ExpiresAt:        time.Now().UTC().Add(time.Hour),
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.CompleteRegistration(
		context.Background(),
		"valid-token",
		"",
	)

	if !errors.Is(err, registration.ErrPasswordRequired) {
		t.Fatalf(
			"expected ErrPasswordRequired, got %v",
			err,
		)
	}

	if repository.completeCalled {
		t.Fatal("complete registration repository must not be called")
	}
}

func TestCompleteRegistrationRejectsTooLongPassword(t *testing.T) {
	repository := &fakeRepository{
		findToken: &registration.VerificationToken{
			ID:               1,
			InvitationEmail:  "test@example.com",
			CompanyType:      registration.CompanyTypeBrand,
			RegistrationPlan: registration.RegistrationPlanBrandFree,
			ExpiresAt:        time.Now().UTC().Add(time.Hour),
		},
	}

	service := registration.NewService(
		repository,
		nil,
	)

	password := string(bytes.Repeat(
		[]byte("a"),
		73,
	))

	_, err := service.CompleteRegistration(
		context.Background(),
		"valid-token",
		password,
	)

	if !errors.Is(err, registration.ErrPasswordTooLong) {
		t.Fatalf(
			"expected ErrPasswordTooLong, got %v",
			err,
		)
	}

	if repository.completeCalled {
		t.Fatal("complete registration repository must not be called")
	}
}

func TestCompleteRegistrationRejectsInvalidToken(t *testing.T) {
	repository := &fakeRepository{
		findErr: registration.ErrVerificationTokenNotFound,
	}

	service := registration.NewService(
		repository,
		nil,
	)

	_, err := service.CompleteRegistration(
		context.Background(),
		"invalid-token",
		"password123",
	)

	if !errors.Is(err, registration.ErrVerificationTokenInvalid) {
		t.Fatalf(
			"expected ErrVerificationTokenInvalid, got %v",
			err,
		)
	}

	if repository.completeCalled {
		t.Fatal("complete registration repository must not be called")
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}