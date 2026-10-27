package adminauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"backend/internal/httpinput"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrCredentials = errors.New("invalid credentials")
var ErrInput = errors.New("invalid input")

type Service struct{ Repository *Repository }

func (s *Service) Create(ctx context.Context, email, password string) error {
	email, err := httpinput.Email(email)
	if err != nil || len(password) < 12 || len(password) > 72 {
		return ErrInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.Repository.Create(ctx, email, string(hash))
}
func (s *Service) Login(ctx context.Context, email, password string) (User, string, time.Time, error) {
	email, err := httpinput.Email(email)
	if err != nil || len(password) == 0 || len(password) > 72 {
		return User{}, "", time.Time{}, ErrCredentials
	}
	u, err := s.Repository.Find(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return User{}, "", time.Time{}, ErrCredentials
	}
	if err != nil {
		return User{}, "", time.Time{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)) != nil {
		return User{}, "", time.Time{}, ErrCredentials
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return User{}, "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expiry := time.Now().UTC().Add(8 * time.Hour)
	err = s.Repository.Session(ctx, u.ID, token, expiry)
	return u, token, expiry, err
}

// Dummy hash is generated once at startup, never an account credential.
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-only-not-an-account"), bcrypt.DefaultCost)
	return string(h)
}()

type contextKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if len(token) != 43 {
		return User{}, ErrCredentials
	}
	return s.Repository.Authenticate(ctx, token)
}
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.Repository.Logout(ctx, token)
}
