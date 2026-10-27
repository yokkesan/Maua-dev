BEGIN;
CREATE TABLE admin_users (
 id BIGSERIAL PRIMARY KEY,
 email VARCHAR(254) NOT NULL UNIQUE,
 password_hash TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE admin_sessions (
 token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash)=32),
 admin_user_id BIGINT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
 expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX admin_sessions_user_idx ON admin_sessions(admin_user_id);
CREATE INDEX admin_sessions_expiry_idx ON admin_sessions(expires_at);
CREATE TABLE password_reset_tokens (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX password_reset_user_idx ON password_reset_tokens(user_id);
CREATE TABLE notification_events (
 id BIGSERIAL PRIMARY KEY,
 event_key TEXT NOT NULL,
 user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
 recipient VARCHAR(254) NOT NULL,
 subject TEXT NOT NULL,
 body TEXT NOT NULL,
 encrypted BOOLEAN NOT NULL DEFAULT FALSE,
 reset_token_id BIGINT REFERENCES password_reset_tokens(id) ON DELETE CASCADE,
 state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','sending','sent','failed','unknown','cancelled')),
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 claimed_at TIMESTAMPTZ,
 sent_at TIMESTAMPTZ,
 last_error TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(event_key,recipient)
);
CREATE INDEX notification_ready_idx ON notification_events(state,next_attempt_at,id);
CREATE TABLE review_decisions (
 id BIGSERIAL PRIMARY KEY,
 entity TEXT NOT NULL CHECK (entity IN ('brands','shops')),
 target_id BIGINT NOT NULL,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 admin_user_id BIGINT NOT NULL REFERENCES admin_users(id),
 approval SMALLINT NOT NULL CHECK (approval IN (1,2)),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(entity,target_id)
);
-- Persistent request buckets avoid an in-process, per-instance rate-limit bypass.
CREATE TABLE security_request_limits (
 scope TEXT NOT NULL,
 key_hash BYTEA NOT NULL,
 window_start TIMESTAMPTZ NOT NULL,
 count INTEGER NOT NULL,
 PRIMARY KEY(scope,key_hash)
);
COMMIT;
