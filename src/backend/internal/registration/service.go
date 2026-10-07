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

	"golang.org/x/crypto/bcrypt"
)

const (
	tokenByteLength = 32
	tokenLifetime   = 24 * time.Hour

	maxPasswordBytes = 72
)

var (
	ErrInvalidEmail              = errors.New("invalid email")
	ErrInvalidCompanyType        = errors.New("invalid company type")
	ErrInvalidPlan               = errors.New("invalid registration plan")
	ErrCompanyNameRequired       = errors.New("company name is required")
	ErrInputTooLong              = errors.New("input is too long")
	ErrVerificationTokenInvalid  = errors.New("verification token is invalid")
	ErrVerificationTokenExpired  = errors.New("verification token has expired")
	ErrVerificationTokenRevoked  = errors.New("verification token has been revoked")
	ErrVerificationTokenConsumed = errors.New("verification token has already been used")
	ErrPasswordRequired          = errors.New("password is required")
	ErrPasswordTooLong           = errors.New("password is too long")
)

type Service struct {
	repository Repository
	mailer     VerificationMailer
}

type VerificationResult struct {
	Token     string
	ExpiresAt time.Time
}

func NewService(
	repository Repository,
	mailer VerificationMailer,
) *Service {
	return &Service{
		repository: repository,
		mailer:     mailer,
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
		return nil, fmt.Errorf(
			"generate signup token: %w",
			err,
		)
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
		return nil, fmt.Errorf(
			"create verification token: %w",
			err,
		)
	}

	if s.mailer != nil {
		if err := s.mailer.SendVerificationEmail(
			ctx,
			input.Email,
			rawToken,
		); err != nil {
			return nil, fmt.Errorf(
				"send verification email: %w",
				err,
			)
		}
	}

	return &VerificationResult{
		Token:     rawToken,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) VerifyToken(
	ctx context.Context,
	rawToken string,
) (*VerificationToken, error) {
	rawToken = strings.TrimSpace(rawToken)

	if rawToken == "" {
		return nil, ErrVerificationTokenInvalid
	}

	tokenHash := sha256.Sum256([]byte(rawToken))

	token, err := s.repository.FindVerificationTokenByHash(
		ctx,
		tokenHash[:],
	)
	if err != nil {
		if errors.Is(
			err,
			ErrVerificationTokenNotFound,
		) {
			return nil, ErrVerificationTokenInvalid
		}

		return nil, fmt.Errorf(
			"find verification token: %w",
			err,
		)
	}

	if token.RevokedAt != nil {
		return nil, ErrVerificationTokenRevoked
	}

	if token.ConsumedAt != nil {
		return nil, ErrVerificationTokenConsumed
	}

	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, ErrVerificationTokenExpired
	}

	return token, nil
}

func (s *Service) CompleteRegistration(
	ctx context.Context,
	rawToken string,
	password string,
) (*CompleteRegistrationResult, error) {
	token, err := s.VerifyToken(
		ctx,
		rawToken,
	)
	if err != nil {
		return nil, err
	}

	if password == "" {
		return nil, ErrPasswordRequired
	}

	if len([]byte(password)) > maxPasswordBytes {
		return nil, ErrPasswordTooLong
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"hash password: %w",
			err,
		)
	}

	result, err := s.repository.CompleteRegistration(
		ctx,
		CompleteRegistrationParams{
			TokenID: token.ID,

			Email:        token.InvitationEmail,
			PasswordHash: string(passwordHash),

			CompanyName:            token.CompanyName,
			CompanyPhonetic:        token.CompanyPhonetic,
			CompanyPostCode:        token.CompanyPostCode,
			CompanyAddress:         token.CompanyAddress,
			CompanyTel:             token.CompanyTel,
			CompanyType:            token.CompanyType,
			RepresentativeName:     token.RepresentativeName,
			RepresentativePhonetic: token.RepresentativePhonetic,
			RegistrationPlan:       token.RegistrationPlan,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"complete registration: %w",
			err,
		)
	}

	return result, nil
}

func validateVerificationInput(
	input VerificationInput,
) error {
	if len(input.Email) == 0 ||
		len(input.Email) > 254 {
		return ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(input.Email)
	if err != nil ||
		parsed.Address != input.Email {
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

func generateToken() (
	string,
	error,
) {
	buf := make(
		[]byte,
		tokenByteLength,
	)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}