package registration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

const (
	tokenByteLength = 32
	tokenLifetime   = 24 * time.Hour
)

var (
	ErrInvalidEmail        = errors.New("invalid email")
	ErrInvalidCompanyType  = errors.New("invalid company type")
	ErrInvalidPlan         = errors.New("invalid registration plan")
	ErrCompanyNameRequired = errors.New("company name is required")
	ErrInputTooLong        = errors.New("input is too long")
)

type Service struct {
	repository Repository
}

type VerificationResult struct {
	Token     string
	ExpiresAt time.Time
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) CreateVerification(
	ctx context.Context,
	input VerificationInput,
) (*VerificationResult, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if err := validateVerificationInput(input); err != nil {
		return nil, err
	}

	rawToken, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate signup token: %w", err)
	}

	tokenHash := sha256.Sum256([]byte(rawToken))
	expiresAt := time.Now().UTC().Add(tokenLifetime)

	err = s.repository.CreateVerificationToken(
		ctx,
		CreateTokenParams{
			VerificationInput: input,
			TokenHash:         tokenHash[:],
			ExpiresAt:         expiresAt,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create verification token: %w", err)
	}

	return &VerificationResult{
		Token:     rawToken,
		ExpiresAt: expiresAt,
	}, nil
}

func validateVerificationInput(input VerificationInput) error {
	if len(input.Email) == 0 || len(input.Email) > 254 {
		return ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || parsed.Address != input.Email {
		return ErrInvalidEmail
	}

	switch input.CompanyType {
	case CompanyTypeShop:
		if input.RegistrationPlan != RegistrationPlanShopPersonal &&
			input.RegistrationPlan != RegistrationPlanShopUnit {
			return ErrInvalidPlan
		}

	case CompanyTypeBrand:
		if input.RegistrationPlan != RegistrationPlanBrandFree {
			return ErrInvalidPlan
		}

	default:
		return ErrInvalidCompanyType
	}

	if input.RegistrationPlan != RegistrationPlanShopPersonal &&
		strings.TrimSpace(input.CompanyName) == "" {
		return ErrCompanyNameRequired
	}

	if len(input.CompanyName) > 255 ||
		len(input.CompanyPhonetic) > 255 ||
		len(input.CompanyPostCode) > 32 ||
		len(input.CompanyAddress) > 500 ||
		len(input.CompanyTel) > 32 ||
		len(input.RepresentativeName) > 255 ||
		len(input.RepresentativePhonetic) > 255 {
		return ErrInputTooLong
	}

	return nil
}

func generateToken() (string, error) {
	buf := make([]byte, tokenByteLength)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}