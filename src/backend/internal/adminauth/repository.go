package adminauth

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Hash  string `json:"-"`
}
type Repository struct{ DB *pgxpool.Pool }

func (r *Repository) Find(ctx context.Context, email string) (User, error) {
	var u User
	err := r.DB.QueryRow(ctx, `SELECT id,email,password_hash FROM admin_users WHERE email=$1`, email).Scan(&u.ID, &u.Email, &u.Hash)
	return u, err
}
func (r *Repository) Create(ctx context.Context, email, hash string) error {
	_, err := r.DB.Exec(ctx, `INSERT INTO admin_users(email,password_hash) VALUES ($1,$2)`, email, hash)
	return err
}
func (r *Repository) Session(ctx context.Context, id int64, token string, expiry time.Time) error {
	h := sha256.Sum256([]byte(token))
	_, err := r.DB.Exec(ctx, `INSERT INTO admin_sessions(admin_user_id,token_hash,expires_at) VALUES ($1,$2,$3)`, id, h[:], expiry)
	return err
}
func (r *Repository) Authenticate(ctx context.Context, token string) (User, error) {
	h := sha256.Sum256([]byte(token))
	var u User
	err := r.DB.QueryRow(ctx, `SELECT u.id,u.email FROM admin_users u JOIN admin_sessions s ON s.admin_user_id=u.id WHERE s.token_hash=$1 AND s.expires_at>NOW()`, h[:]).Scan(&u.ID, &u.Email)
	return u, err
}
func (r *Repository) Logout(ctx context.Context, token string) error {
	h := sha256.Sum256([]byte(token))
	_, err := r.DB.Exec(ctx, `DELETE FROM admin_sessions WHERE token_hash=$1`, h[:])
	return err
}
