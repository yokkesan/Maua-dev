package passwordreset

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"backend/internal/httpinput"

	"golang.org/x/crypto/bcrypt"
)

type Service struct{ Repository *Repository }

func (s *Service) Request(ctx context.Context, email string) error {
	email, err := httpinput.Email(email)
	if err != nil {
		return ErrInput
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return err
	}
	return s.Repository.Request(ctx, email, base64.RawURLEncoding.EncodeToString(b))
}
func (s *Service) Reset(ctx context.Context, token, password string) error {
	if len(token) != 43 {
		return ErrToken
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != 32 {
		return ErrToken
	}
	if len(password) < 8 || len(password) > 72 {
		return ErrInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.Repository.Reset(ctx, token, string(hash))
}
