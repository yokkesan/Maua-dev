package notification

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"backend/internal/httpinput"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	AdminEmails                     []string
	AppURL, ResetURL, EncryptionKey string
}

func FromEnv() Config {
	return Config{AdminEmails: strings.Split(os.Getenv("ADMIN_NOTIFICATION_EMAILS"), ","), AppURL: os.Getenv("APP_URL"), ResetURL: os.Getenv("PASSWORD_RESET_URL"), EncryptionKey: os.Getenv("NOTIFICATION_ENCRYPTION_KEY")}
}

type Message struct {
	To, Subject, Body string
	ID                int64
}

func Enqueue(ctx context.Context, tx pgx.Tx, key string, userID int64, m Message, encrypted bool, resetID *int64) error {
	recipient, err := httpinput.Email(m.To)
	if err != nil {
		return errors.New("invalid notification recipient")
	}
	_, err = tx.Exec(ctx, `INSERT INTO notification_events(event_key,user_id,recipient,subject,body,encrypted,reset_token_id) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(event_key,recipient) DO NOTHING`, key, userID, recipient, m.Subject, m.Body, encrypted, resetID)
	return err
}

const footer = "\n\n──────────────────────────────────\n株式会社 Freeliving.\nAUMA運営事業部\nmail： support@auma.jp\nURL：https://auma.jp\nこのメールへの返信は出来ませんのでご了承ください。\n本メールにお心あたりのない方は破棄して頂きますよう御願い致します。\n──────────────────────────────────\n"

func Submission(ctx context.Context, tx pgx.Tx, config Config, kind string, id, companyID, actorID int64) error {
	var email, name string
	if err := tx.QueryRow(ctx, `SELECT email FROM users WHERE id=$1 AND company_id=$2`, actorID, companyID).Scan(&email); err != nil {
		return err
	}
	if kind != "brands" && kind != "shops" {
		return errors.New("invalid entity")
	}
	if err := tx.QueryRow(ctx, "SELECT name FROM "+kind+" WHERE id=$1 AND company_id=$2", id, companyID).Scan(&name); err != nil {
		return err
	}
	key := fmt.Sprintf("submission:%s:%d", kind, id)
	if err := Enqueue(ctx, tx, key, actorID, Message{To: email, Subject: "AUMAへのご登録ありがとうございます", Body: "審査登録が完了しました。運営審査後メールにてご連絡いたします。" + footer}, false, nil); err != nil {
		return err
	}
	if len(config.AdminEmails) == 0 {
		return errors.New("admin notification recipients required")
	}
	label := "ブランド"
	if kind == "shops" {
		label = "ショップ"
	}
	for _, recipient := range config.AdminEmails {
		body := fmt.Sprintf("%s名：【%s】から新規登録がありました。\n下記のURLで審査状況を確認してください。\n\nURL： %s/info/%s/%d", label, name, strings.TrimRight(config.AppURL, "/"), kind, id) + footer
		if err := Enqueue(ctx, tx, key+":operation", actorID, Message{To: recipient, Subject: "AUMA 新規ご登録通知メール", Body: body}, false, nil); err != nil {
			return err
		}
	}
	return nil
}
func Decision(ctx context.Context, tx pgx.Tx, kind string, id, companyID int64, approval int16) error {
	rows, err := tx.Query(ctx, `SELECT id,email FROM users WHERE company_id=$1 AND role=0 ORDER BY id LIMIT 1`, companyID)
	if err != nil {
		return err
	}
	type recipient struct {
		id    int64
		email string
	}
	var recipients []recipient
	for rows.Next() {
		var r recipient
		if err := rows.Scan(&r.id, &r.email); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return errors.New("company manager required")
	}
	subject := "AUMA 運営審査が完了しました。"
	body := "AUMAへのご登録ありがとうございます。運営審査が完了しました。"
	if approval == 2 {
		subject = "AUMA 運営審査が完了できませんでした。"
		body = "AUMAへのご登録ありがとうございます。運営審査が完了できませんでした。"
	} else {
		body += "\n登録時に設定したパスワードでログインしてください。"
	}
	for _, r := range recipients {
		if err := Enqueue(ctx, tx, fmt.Sprintf("decision:%s:%d", kind, id), r.id, Message{To: r.email, Subject: subject, Body: body + footer}, false, nil); err != nil {
			return err
		}
	}
	return nil
}
func seal(key, text string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) != 32 {
		return "", errors.New("notification encryption key must be base64 encoded 32 bytes")
	}
	block, err := aes.NewCipher(b)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(text), nil)), nil
}
func open(key, text string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) != 32 {
		return "", errors.New("invalid notification encryption key")
	}
	block, _ := aes.NewCipher(b)
	g, _ := cipher.NewGCM(block)
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil || len(data) < g.NonceSize() {
		return "", errors.New("invalid encrypted notification")
	}
	plain, err := g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], nil)
	return string(plain), err
}
func Encrypt(key, text string) (string, error) { return seal(key, text) }

// Rate-limit keys are hashed to avoid storing submitted email addresses or IPs.
func Allow(ctx context.Context, db *pgxpool.Pool, scope, key string, limit int) (bool, error) {
	h := sha256.Sum256([]byte(key))
	var count int
	err := db.QueryRow(ctx, `INSERT INTO security_request_limits(scope,key_hash,window_start,count) VALUES ($1,$2,NOW(),1) ON CONFLICT(scope,key_hash) DO UPDATE SET count=CASE WHEN security_request_limits.window_start<NOW()-INTERVAL '1 minute' THEN 1 ELSE security_request_limits.count+1 END,window_start=CASE WHEN security_request_limits.window_start<NOW()-INTERVAL '1 minute' THEN NOW() ELSE security_request_limits.window_start END RETURNING count`, scope, h[:]).Scan(&count)
	return count <= limit, err
}

type Repository struct{ DB *pgxpool.Pool }
type claimedEvent struct {
	ID        int64
	Attempts  int
	Message   Message
	Encrypted bool
}

func (r *Repository) List(ctx context.Context, before int64) ([]Event, error) {
	rows, err := r.DB.Query(ctx, `SELECT id,event_key,recipient,subject,state,attempts,last_error,created_at,sent_at FROM notification_events WHERE ($1::bigint=0 OR id<$1) ORDER BY id DESC LIMIT 50`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Key, &e.Recipient, &e.Subject, &e.State, &e.Attempts, &e.LastError, &e.CreatedAt, &e.SentAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
func (r *Repository) claim(ctx context.Context) (*claimedEvent, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// An abandoned claim may have delivered its message before the process died.
	if _, err = tx.Exec(ctx, `UPDATE notification_events SET state='unknown',last_error='worker_interrupted',body=CASE WHEN encrypted THEN '' ELSE body END WHERE state='sending' AND claimed_at<NOW()-INTERVAL '5 minutes'`); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE notification_events e SET state='cancelled',body='' WHERE state IN ('pending','failed') AND reset_token_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM password_reset_tokens t WHERE t.id=e.reset_token_id AND t.revoked_at IS NULL AND t.consumed_at IS NULL AND t.expires_at>NOW())`); err != nil {
		return nil, err
	}
	var id int64
	var attempts int
	var m Message
	var encrypted bool
	err = tx.QueryRow(ctx, `UPDATE notification_events SET state='sending',claimed_at=NOW(),attempts=attempts+1 WHERE id=(SELECT id FROM notification_events WHERE state IN ('pending','failed') AND next_attempt_at<=NOW() ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,recipient,subject,body,encrypted,attempts`).Scan(&id, &m.To, &m.Subject, &m.Body, &encrypted, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	m.ID = id
	return &claimedEvent{ID: id, Attempts: attempts, Message: m, Encrypted: encrypted}, nil
}
func (r *Repository) finish(ctx context.Context, e *claimedEvent, state, code string) error {
	_, err := r.DB.Exec(ctx, `UPDATE notification_events SET state=$1,last_error=NULLIF($2,''),sent_at=CASE WHEN $1='sent' THEN NOW() ELSE NULL END,next_attempt_at=NOW()+INTERVAL '5 minutes',body=CASE WHEN encrypted AND $1 IN ('sent','unknown') THEN '' ELSE body END WHERE id=$3 AND state='sending' AND attempts=$4`, state, code, e.ID, e.Attempts)
	return err
}

// Validate is used at process startup; requests never silently omit notifications.
func (c Config) Validate() error {
	if len(c.AdminEmails) == 0 {
		return errors.New("ADMIN_NOTIFICATION_EMAILS is required")
	}
	for _, email := range c.AdminEmails {
		if _, err := httpinput.Email(email); err != nil {
			return errors.New("invalid ADMIN_NOTIFICATION_EMAILS")
		}
	}
	for _, raw := range []string{c.AppURL, c.ResetURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return errors.New("APP_URL and PASSWORD_RESET_URL must use HTTPS")
		}
	}
	_, err := seal(c.EncryptionKey, "")
	return err
}

// ResetBody follows Laravel's public, 60-minute reset notification template.
func ResetBody(link string) string {
	return "下記のURLからパスワードを再設定することができます。\n再設定用URL： " + link + "\nURLの有効期限： メール発行から60分間\n\n本メールにお心当たりのない場合は、お手数ですが本メールを破棄してくださいますようお願い致します。" + footer
}
