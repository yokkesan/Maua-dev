CREATE TABLE signup_tokens (
    id BIGSERIAL PRIMARY KEY,

    invitation_email VARCHAR(254) NOT NULL,

    -- 生トークンは保存しない
    token_hash BYTEA NOT NULL UNIQUE,

    company_name VARCHAR(255),
    company_phonetic VARCHAR(255),
    company_post_code VARCHAR(32),
    company_address VARCHAR(500),
    company_tel VARCHAR(32),

    -- shop: 0 / brand: 1
    company_type SMALLINT NOT NULL,

    representative_name VARCHAR(255),
    representative_phonetic VARCHAR(255),

    -- brand: 0
    -- shop: 10 personal / 11 unit
    registration_plan SMALLINT NOT NULL,

    expires_at TIMESTAMPTZ NOT NULL,

    -- 本登録完了
    consumed_at TIMESTAMPTZ,

    -- 再発行等による無効化
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT signup_tokens_company_type_check
        CHECK (company_type IN (0, 1)),

    CONSTRAINT signup_tokens_registration_plan_check
        CHECK (
            (company_type = 1 AND registration_plan = 0)
            OR
            (company_type = 0 AND registration_plan IN (10, 11))
        )
);

CREATE INDEX idx_signup_tokens_email
    ON signup_tokens (invitation_email);

CREATE INDEX idx_signup_tokens_expires_at
    ON signup_tokens (expires_at);