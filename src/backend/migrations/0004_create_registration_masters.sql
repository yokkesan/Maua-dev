-- =========================================================
-- カテゴリーマスター
-- =========================================================

CREATE TABLE first_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE second_categories (
    id BIGSERIAL PRIMARY KEY,
    first_category_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT second_categories_first_category_fk
        FOREIGN KEY (first_category_id)
        REFERENCES first_categories(id)
);

CREATE TABLE third_categories (
    id BIGSERIAL PRIMARY KEY,
    second_category_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT third_categories_second_category_fk
        FOREIGN KEY (second_category_id)
        REFERENCES second_categories(id)
);

CREATE INDEX idx_second_categories_first_category_id
    ON second_categories (first_category_id);

CREATE INDEX idx_third_categories_second_category_id
    ON third_categories (second_category_id);


-- =========================================================
-- 地域マスター
-- =========================================================

CREATE TABLE area_regions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE area_prefectures (
    id BIGSERIAL PRIMARY KEY,
    area_region_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT area_prefectures_region_fk
        FOREIGN KEY (area_region_id)
        REFERENCES area_regions(id)
);

CREATE TABLE area_cities (
    id BIGSERIAL PRIMARY KEY,
    area_prefecture_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT area_cities_prefecture_fk
        FOREIGN KEY (area_prefecture_id)
        REFERENCES area_prefectures(id)
);

CREATE INDEX idx_area_prefectures_region_id
    ON area_prefectures (area_region_id);

CREATE INDEX idx_area_cities_prefecture_id
    ON area_cities (area_prefecture_id);


-- =========================================================
-- 価格帯マスター
-- =========================================================

CREATE TABLE price_zones (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE price_zone_guidelines (
    id BIGSERIAL PRIMARY KEY,
    second_id BIGINT NOT NULL,
    third_id BIGINT,
    price_rank SMALLINT NOT NULL,
    price_range VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT price_zone_guidelines_second_fk
        FOREIGN KEY (second_id)
        REFERENCES second_categories(id),

    CONSTRAINT price_zone_guidelines_third_fk
        FOREIGN KEY (third_id)
        REFERENCES third_categories(id),

    CONSTRAINT price_zone_guidelines_rank_check
        CHECK (price_rank IN (1, 2, 3))
);

CREATE INDEX idx_price_zone_guidelines_second_id
    ON price_zone_guidelines (second_id);

CREATE INDEX idx_price_zone_guidelines_third_id
    ON price_zone_guidelines (third_id);


-- =========================================================
-- ブランド用マスター
-- =========================================================

-- 募集区分
CREATE TABLE sales_recruitments (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 販売形態
CREATE TABLE sales_styles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 注文条件
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 支払方法
CREATE TABLE pay_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);