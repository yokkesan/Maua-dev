package shop_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"backend/internal/notification"

	"github.com/jackc/pgx/v5"
)

func TestSubmissionNotificationEvents(t *testing.T) {
	user, id, cookie := submissionTarget(t)
	requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(id), cookie), 200)
	rows, err := testDB.Query(context.Background(), "SELECT recipient,subject,body,state FROM notification_events WHERE user_id=$1 ORDER BY id", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	messages := []notification.Message{}
	for rows.Next() {
		var m notification.Message
		var state string
		if err := rows.Scan(&m.To, &m.Subject, &m.Body, &state); err != nil {
			t.Fatal(err)
		}
		if state != "pending" {
			t.Fatal("mail sent inside submission transaction")
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	config := notification.FromEnv()
	if len(messages) != 1+len(config.AdminEmails) {
		t.Fatalf("notification count %d", len(messages))
	}
	if messages[0].To != user.Email || messages[0].Subject != "AUMAへのご登録ありがとうございます" || !strings.Contains(messages[0].Body, "審査登録が完了しました。運営審査後メールにてご連絡いたします。") {
		t.Fatal("applicant notification differs from Laravel")
	}
	for i, email := range config.AdminEmails {
		if messages[i+1].To != strings.ToLower(strings.TrimSpace(email)) || messages[i+1].Subject != "AUMA 新規ご登録通知メール" || !strings.Contains(messages[i+1].Body, "下記のURLで審査状況を確認してください。") {
			t.Fatal("operation notification differs from Laravel")
		}
	}
	requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(id), cookie), 409)
	var count int
	if err := testDB.QueryRow(context.Background(), "SELECT count(*) FROM notification_events WHERE user_id=$1", user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(messages) {
		t.Fatal("duplicate submission notification")
	}
}
func TestSubmissionNotificationRollback(t *testing.T) {
	t.Parallel()
	user, id, cookie := submissionTarget(t)
	// The trigger rejects only this fixture actor; other tests remain unaffected.
	before := readSubmissionSnapshot(t, testDB, id)
	// Inject a database error while notification and submission are uncommitted.
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("submission_mail_shop_%s", hex.EncodeToString(nonce[:]))
	quoted := pgx.Identifier{name}.Sanitize()
	query := fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.user_id=%d THEN RAISE EXCEPTION 'test-notification-failure'; END IF; RETURN NEW; END $$`, quoted, user.ID)
	if _, err := testDB.Exec(context.Background(), query); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := testDB.Exec(context.Background(), "DROP FUNCTION "+quoted+"() CASCADE"); err != nil {
			t.Error(err)
		}
	})
	if _, err := testDB.Exec(context.Background(), "CREATE TRIGGER "+quoted+" BEFORE INSERT ON notification_events FOR EACH ROW EXECUTE FUNCTION "+quoted+"()"); err != nil {
		t.Fatal(err)
	}
	requireSubmissionHTTP(t, submissionRequest("POST", fmt.Sprint(id), cookie), 500)
	if after := readSubmissionSnapshot(t, testDB, id); !after.UpdatedAt.Equal(before.UpdatedAt) || after.State.SubmittedAt != nil {
		t.Fatal("notification failure partially submitted")
	}
	var count int
	if err := testDB.QueryRow(context.Background(), "SELECT count(*) FROM notification_events WHERE user_id=$1", user.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed submission left notification")
	}
}
