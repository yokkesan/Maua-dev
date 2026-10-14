ALTER TABLE users
ADD COLUMN role SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE users
ADD CONSTRAINT users_role_check
CHECK (role IN (0, 1));

CREATE TABLE user_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_sessions_user_id_fk
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT user_sessions_token_hash_unique
        UNIQUE (token_hash)
);

CREATE INDEX idx_user_sessions_user_id
    ON user_sessions (user_id);

CREATE INDEX idx_user_sessions_expires_at
    ON user_sessions (expires_at);