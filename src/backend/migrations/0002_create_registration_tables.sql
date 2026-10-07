CREATE TABLE companies (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255),
    phonetic VARCHAR(255),
    post_code VARCHAR(32),
    address VARCHAR(500),
    tel VARCHAR(32),
    company_type SMALLINT NOT NULL,
    representative_name VARCHAR(255),
    representative_phonetic VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT companies_company_type_check
        CHECK (company_type IN (0, 1))
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL,
    email VARCHAR(254) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    user_type SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_company_id_fk
        FOREIGN KEY (company_id)
        REFERENCES companies(id)
        ON DELETE CASCADE,

    CONSTRAINT users_email_unique
        UNIQUE (email),

    CONSTRAINT users_user_type_check
        CHECK (user_type IN (1, 2))
);

CREATE TABLE brands (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    phonetic VARCHAR(255),
    tel VARCHAR(32),
    email VARCHAR(254),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT brands_company_id_fk
        FOREIGN KEY (company_id)
        REFERENCES companies(id)
        ON DELETE CASCADE,

    CONSTRAINT brands_company_id_unique
        UNIQUE (company_id)
);

CREATE TABLE shops (
    id BIGSERIAL PRIMARY KEY,
    company_id BIGINT NOT NULL,
    name VARCHAR(255),
    phonetic VARCHAR(255),
    tel VARCHAR(32),
    email VARCHAR(254),
    registration_plan SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT shops_company_id_fk
        FOREIGN KEY (company_id)
        REFERENCES companies(id)
        ON DELETE CASCADE,

    CONSTRAINT shops_company_id_unique
        UNIQUE (company_id),

    CONSTRAINT shops_registration_plan_check
        CHECK (registration_plan IN (10, 11))
);

CREATE INDEX idx_users_company_id
    ON users (company_id);

CREATE INDEX idx_brands_company_id
    ON brands (company_id);

CREATE INDEX idx_shops_company_id
    ON shops (company_id);