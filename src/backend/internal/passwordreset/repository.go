package passwordreset

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"

	"backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrToken = errors.New("invalid or expired token")
var ErrInput = errors.New("invalid input")

type Repository struct {
	DB     *pgxpool.Pool
	Config notification.Config
}

func (r *Repository) Request(ctx context.Context, email, token string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID int64
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email=$1 FOR UPDATE`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	u, err := url.Parse(r.Config.ResetURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("HTTPS password reset URL required")
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	body, err := notification.Encrypt(r.Config.EncryptionKey, notification.ResetBody(u.String()))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET revoked_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_events SET state='cancelled',body='' WHERE reset_token_id IN (SELECT id FROM password_reset_tokens WHERE user_id=$1) AND state IN ('pending','failed')`, userID); err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(token))
	var id int64
	if err = tx.QueryRow(ctx, `INSERT INTO password_reset_tokens(user_id,token_hash,expires_at) VALUES ($1,$2,NOW()+INTERVAL '60 minutes') RETURNING id`, userID, hash[:]).Scan(&id); err != nil {
		return err
	}
	if err = notification.Enqueue(ctx, tx, fmt.Sprintf("password-reset:%d", id), userID, notification.Message{To: email, Subject: "AUMA パスワード再設定URL", Body: body}, true, &id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) Reset(ctx context.Context, token, passwordHash string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	hash := sha256.Sum256([]byte(token))
	var userID, id int64
	// Lock the same user before token mutation, matching issuance lock order.
	err = tx.QueryRow(ctx, `SELECT u.id,t.id FROM users u JOIN password_reset_tokens t ON t.user_id=u.id WHERE t.token_hash=$1 FOR UPDATE OF u`, hash[:]).Scan(&userID, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrToken
	}
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE password_reset_tokens SET consumed_at=NOW() WHERE id=$1 AND revoked_at IS NULL AND consumed_at IS NULL AND expires_at>NOW()`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrToken
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2`, passwordHash, userID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE password_reset_tokens SET revoked_at=NOW() WHERE user_id=$1 AND id<>$2 AND consumed_at IS NULL AND revoked_at IS NULL`, userID, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_events SET state='cancelled',body='' WHERE reset_token_id IN (SELECT id FROM password_reset_tokens WHERE user_id=$1) AND state IN ('pending','failed')`, userID); err != nil {
		return err
	}
	// A password reset revokes all old general-user sessions, never admin sessions.
	if _, err = tx.Exec(ctx, `DELETE FROM user_sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
