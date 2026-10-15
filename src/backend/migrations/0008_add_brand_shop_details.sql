BEGIN;

-- =========================================================
-- 1. 会社詳細
-- =========================================================

ALTER TABLE companies
    ADD COLUMN google_map_url TEXT,
    ADD COLUMN facebook TEXT,
    ADD COLUMN twitter TEXT,
    ADD COLUMN instagram TEXT,
    ADD COLUMN line TEXT,
    ADD COLUMN description TEXT;


-- =========================================================
-- 2. ブランド詳細
-- =========================================================

ALTER TABLE brands
    ADD COLUMN concept TEXT,
    ADD COLUMN top_news TEXT,
    ADD COLUMN other_brand TEXT,

    ADD COLUMN target_min VARCHAR(20),
    ADD COLUMN target_max VARCHAR(20),

    ADD COLUMN web_url TEXT,
    ADD COLUMN net_shop SMALLINT,

    ADD COLUMN contact_person_name VARCHAR(255),
    ADD COLUMN contact_person_phonetic VARCHAR(255),
    ADD COLUMN contact_person_tel VARCHAR(50),

    ADD COLUMN facebook TEXT,
    ADD COLUMN twitter TEXT,
    ADD COLUMN instagram TEXT,
    ADD COLUMN line TEXT,

    ADD COLUMN price_min VARCHAR(50),
    ADD COLUMN price_max VARCHAR(50),

    ADD COLUMN base TEXT,

    ADD COLUMN rate_dm_form SMALLINT,
    ADD COLUMN rate_min VARCHAR(50),
    ADD COLUMN rate_max VARCHAR(50),

    ADD COLUMN box_price_min VARCHAR(50),
    ADD COLUMN box_price_max VARCHAR(50),

    ADD COLUMN one_gram VARCHAR(50),
    ADD COLUMN gram_price_min VARCHAR(50),
    ADD COLUMN gram_price_max VARCHAR(50),

    ADD COLUMN price_free_text TEXT,
    ADD COLUMN lot_free_text TEXT,

    ADD COLUMN price_zone_id BIGINT
        REFERENCES price_zones(id),

    ADD COLUMN approval SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN status SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE brands
    ADD CONSTRAINT brands_net_shop_check
        CHECK (net_shop IS NULL OR net_shop BETWEEN 0 AND 4),

    ADD CONSTRAINT brands_rate_dm_form_check
        CHECK (rate_dm_form IS NULL OR rate_dm_form BETWEEN 0 AND 5),

    ADD CONSTRAINT brands_approval_check
        CHECK (approval IN (0, 1, 2)),

    ADD CONSTRAINT brands_status_check
        CHECK (status IN (0, 1));


-- =========================================================
-- 3. ショップ詳細
-- =========================================================

ALTER TABLE shops
    ADD COLUMN concept TEXT,

    ADD COLUMN target_min VARCHAR(20),
    ADD COLUMN target_max VARCHAR(20),

    ADD COLUMN web_url TEXT,
    ADD COLUMN net_shop SMALLINT,

    ADD COLUMN contact_person_name VARCHAR(255),
    ADD COLUMN contact_person_phonetic VARCHAR(255),
    ADD COLUMN contact_person_tel VARCHAR(50),

    ADD COLUMN facebook TEXT,
    ADD COLUMN twitter TEXT,
    ADD COLUMN instagram TEXT,
    ADD COLUMN line TEXT,

    ADD COLUMN address TEXT,
    ADD COLUMN handling_brand TEXT,

    ADD COLUMN price_zone_id BIGINT
        REFERENCES price_zones(id),

    ADD COLUMN approval SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN status SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE shops
    ADD CONSTRAINT shops_net_shop_check
        CHECK (net_shop IS NULL OR net_shop BETWEEN 0 AND 4),

    ADD CONSTRAINT shops_approval_check
        CHECK (approval IN (0, 1, 2)),

    ADD CONSTRAINT shops_status_check
        CHECK (status IN (0, 1));


-- =========================================================
-- 4. カテゴリーの組み合わせ
-- =========================================================

CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,

    first_id BIGINT NOT NULL
        REFERENCES first_categories(id),

    second_id BIGINT NOT NULL
        REFERENCES second_categories(id),

    third_id BIGINT
        REFERENCES third_categories(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (second_id, third_id)
);

-- PostgreSQLではNULLを含むUNIQUE制約だけでは
-- 第2階層のみの重複を防げないため、別途制約する。

CREATE UNIQUE INDEX categories_second_only_unique
    ON categories(second_id)
    WHERE third_id IS NULL;


-- =========================================================
-- 5. ブランドカテゴリー
-- =========================================================

CREATE TABLE brand_categories (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    category_id BIGINT NOT NULL
        REFERENCES categories(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (brand_id, category_id)
);


-- =========================================================
-- 6. ショップカテゴリー
-- =========================================================

CREATE TABLE shop_categories (
    shop_id BIGINT NOT NULL
        REFERENCES shops(id) ON DELETE CASCADE,

    category_id BIGINT NOT NULL
        REFERENCES categories(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (shop_id, category_id)
);


-- =========================================================
-- 7. ブランド募集地域
--
-- 新AUMAでは市区町村IDを直接保持する。
-- マスターデータはGoコード・CSVで管理しているため、
-- DB外部キーは設定しない。
-- =========================================================

CREATE TABLE brand_areas (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    city_id BIGINT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (brand_id, city_id)
);


-- =========================================================
-- 8. ブランドBtoB募集区分
-- =========================================================

CREATE TABLE brand_sales_recruitments (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    sales_recruitment_id BIGINT NOT NULL,

    PRIMARY KEY (brand_id, sales_recruitment_id)
);


-- =========================================================
-- 9. ブランド販売形態
-- =========================================================

CREATE TABLE brand_sales_styles (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    sales_style_id BIGINT NOT NULL,

    PRIMARY KEY (brand_id, sales_style_id)
);


-- =========================================================
-- 10. ブランド注文条件
-- =========================================================

CREATE TABLE brand_orders (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    order_id BIGINT NOT NULL,

    PRIMARY KEY (brand_id, order_id)
);


-- =========================================================
-- 11. ブランド支払条件
-- =========================================================

CREATE TABLE brand_pay_rules (
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,

    pay_rule_id BIGINT NOT NULL,

    PRIMARY KEY (brand_id, pay_rule_id)
);


COMMIT;