package auth_test

import (
	"backend/internal/auth"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

// Gate the exact boundary after bcrypt validation and before/after the real
// repository transaction. No replacement login or reset implementation is used.
type sessionGateRepository struct {
	auth.Repository
	before, after bool
	reached       chan struct{}
	release       chan struct{}
}

func (r *sessionGateRepository) CreateSession(ctx context.Context, s auth.Session, expectedHash string) error {
	gate := func() error {
		close(r.reached)
		select {
		case <-r.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if r.before {
		if err := gate(); err != nil {
			return err
		}
	}
	err := r.Repository.CreateSession(ctx, s, expectedHash)
	if err == nil && r.after {
		if gateErr := gate(); gateErr != nil {
			return gateErr
		}
	}
	return err
}
func TestLoginAndPasswordResetBothOrderings(t *testing.T) {
	for _, resetFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "reset_commits_before_session", false: "session_commits_before_reset"}[resetFirst], func(t *testing.T) {
			u := createTestUser(t, 2)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			reset := resetService()
			if err := reset.Request(ctx, u.Email); err != nil {
				t.Fatal(err)
			}
			token := dispatchReset(t, u)
			gate := &sessionGateRepository{Repository: auth.NewPostgresRepository(testDB), before: resetFirst, after: !resetFirst, reached: make(chan struct{}), release: make(chan struct{})}
			defer func() {
				select {
				case <-gate.release:
				default:
					close(gate.release)
				}
			}()
			login := auth.NewService(gate)
			type outcome struct {
				result *auth.LoginResult
				err    error
			}
			done := make(chan outcome, 1)
			go func() {
				result, err := login.Login(ctx, auth.LoginInput{Email: u.Email, Password: testPassword})
				done <- outcome{result, err}
			}()
			select {
			case <-gate.reached:
			case <-ctx.Done():
				t.Fatal("login did not reach validated-hash boundary")
			}
			if err := reset.Reset(ctx, token, "replacement-password"); err != nil {
				t.Fatal(err)
			}
			close(gate.release)
			var out outcome
			select {
			case out = <-done:
			case <-ctx.Done():
				t.Fatal("login/reset deadlock")
			}
			if resetFirst {
				if !errors.Is(out.err, auth.ErrInvalidCredentials) || out.result != nil {
					t.Fatal("old hash created session after reset")
				}
			} else {
				if out.err != nil {
					t.Fatal(out.err)
				}
				if _, err := login.Authenticate(ctx, out.result.Token); !errors.Is(err, auth.ErrUnauthorized) {
					t.Fatal("session committed before reset was not revoked")
				}
			}
			if workflowID(t, "SELECT count(*) FROM user_sessions WHERE user_id=$1", u.ID) != 0 {
				t.Fatal("old credential session survived")
			}
			if _, err := login.Login(ctx, auth.LoginInput{Email: u.Email, Password: testPassword}); !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Fatal("old password accepted")
			}
			fresh := auth.NewService(auth.NewPostgresRepository(testDB))
			if _, err := fresh.Login(ctx, auth.LoginInput{Email: u.Email, Password: "replacement-password"}); err != nil {
				t.Fatal("new password rejected")
			}
		})
	}
}

// The reset transaction already owns the row when session issuance arrives.
// Observe PostgreSQL's lock wait instead of relying on a scheduling sleep.
func TestSessionCreationWaitsForResetUserLock(t *testing.T) {
	u := createTestUser(t, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repository := auth.NewPostgresRepository(testDB)
	user, err := repository.FindUserByEmail(ctx, u.Email)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := testDB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", u.ID); err != nil {
		t.Fatal(err)
	}
	config := testDB.Config()
	application := fmt.Sprintf("login_reset_lock_%d", u.ID)
	config.ConnConfig.RuntimeParams["application_name"] = application
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	done := make(chan error, 1)
	go func() {
		done <- auth.NewPostgresRepository(db).CreateSession(ctx, auth.Session{UserID: u.ID, TokenHash: make([]byte, 32), ExpiresAt: time.Now().Add(time.Hour)}, user.PasswordHash)
	}()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	waiting := false
	for !waiting {
		if err := testDB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND datname=current_database() AND wait_event_type='Lock')`, application).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if !waiting {
			select {
			case err := <-done:
				t.Fatalf("session did not wait for user lock: %v", err)
			case <-ticker.C:
			case <-ctx.Done():
				t.Fatal("no user lock wait observed")
			}
		}
	}
	// Model the reset's password update and session revocation while holding its
	// actual user lock. End-to-end reset ordering is covered by the test above.
	hash, err := bcrypt.GenerateFromPassword([]byte("replacement-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE users SET password_hash=$1 WHERE id=$2", string(hash), u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM user_sessions WHERE user_id=$1", u.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatal("stale hash accepted after lock wait")
		}
	case <-ctx.Done():
		t.Fatal("session did not finish")
	}
	if workflowID(t, "SELECT count(*) FROM user_sessions WHERE user_id=$1", u.ID) != 0 {
		t.Fatal("stale session persisted")
	}
}
