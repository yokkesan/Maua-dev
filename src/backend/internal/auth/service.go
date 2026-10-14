package auth

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
	sessionTokenByteLength = 32
	sessionLifetime        = 24 * time.Hour
	maxPasswordBytes       = 72
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrPasswordRequired   = errors.New("password is required")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
)

type Service struct {
	repository Repository
}

func NewService(
	repository Repository,
) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Login(
	ctx context.Context,
	input LoginInput,
) (*LoginResult, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if err := validateLoginInput(input); err != nil {
		return nil, err
	}

	user, err := s.repository.FindUserByEmail(
		ctx,
		input.Email,
	)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, fmt.Errorf(
			"find login user: %w",
			err,
		)
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(input.Password),
	); err != nil {
		return nil, ErrInvalidCredentials
	}

	rawToken, err := generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf(
			"generate session token: %w",
			err,
		)
	}

	tokenHash := sha256.Sum256([]byte(rawToken))
	expiresAt := time.Now().UTC().Add(sessionLifetime)

	err = s.repository.CreateSession(
		ctx,
		Session{
			UserID:    user.ID,
			TokenHash: tokenHash[:],
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create session: %w",
			err,
		)
	}

	return &LoginResult{
		UserID:    user.ID,
		CompanyID: user.CompanyID,
		Email:     user.Email,
		UserType:  user.UserType,
		Role:      user.Role,
		Token:     rawToken,
		ExpiresAt: expiresAt,
	}, nil
}

func validateLoginInput(
	input LoginInput,
) error {
	if input.Email == "" {
		return ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || parsed.Address != input.Email {
		return ErrInvalidEmail
	}

	if input.Password == "" {
		return ErrPasswordRequired
	}

	if len([]byte(input.Password)) > maxPasswordBytes {
		return ErrInvalidCredentials
	}

	return nil
}

func generateSessionToken() (
	string,
	error,
) {
	buf := make(
		[]byte,
		sessionTokenByteLength,
	)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Service) Authenticate(
	ctx context.Context,
	rawToken string,
) (*User, error) {
	rawToken = strings.TrimSpace(rawToken)

	if rawToken == "" {
		return nil, ErrUnauthorized
	}

	tokenHash := sha256.Sum256([]byte(rawToken))

	user, err := s.repository.FindUserBySessionTokenHash(
		ctx,
		tokenHash[:],
	)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, ErrUnauthorized
		}

		return nil, fmt.Errorf(
			"authenticate session: %w",
			err,
		)
	}

	return user, nil
}

func (s *Service) Logout(
	ctx context.Context,
	rawToken string,
) error {
	rawToken = strings.TrimSpace(rawToken)

	if rawToken == "" {
		return nil
	}

	tokenHash := sha256.Sum256([]byte(rawToken))

	if err := s.repository.DeleteSessionByTokenHash(
		ctx,
		tokenHash[:],
	); err != nil {
		return fmt.Errorf(
			"logout session: %w",
			err,
		)
	}

	return nil
}
