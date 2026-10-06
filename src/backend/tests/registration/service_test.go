package registration_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"backend/internal/registration"
)

type fakeRepository struct {
	params registration.CreateTokenParams
	called bool
}

func (r *fakeRepository) CreateVerificationToken(
	ctx context.Context,
	params registration.CreateTokenParams,
) error {
	r.params = params
	r.called = true

	return nil
}

func TestCreateVerificationBrand(t *testing.T) {
	repository := &fakeRepository{}
	service := registration.NewService(repository)

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
}

func TestCreateVerificationRejectsInvalidEmail(t *testing.T) {
	repository := &fakeRepository{}
	service := registration.NewService(repository)

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
}

func TestCreateVerificationRejectsInvalidPlan(t *testing.T) {
	repository := &fakeRepository{}
	service := registration.NewService(repository)

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
}