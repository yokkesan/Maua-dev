BEGIN;

CREATE TABLE brand_price_screenings (
    brand_id BIGINT PRIMARY KEY REFERENCES brands(id) ON DELETE CASCADE,
    first_category_id BIGINT NOT NULL REFERENCES first_categories(id),
    saved_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE brand_price_screening_products (
    brand_id BIGINT NOT NULL REFERENCES brand_price_screenings(brand_id) ON DELETE CASCADE,
    slot_number SMALLINT NOT NULL CHECK (slot_number BETWEEN 1 AND 3),
    second_category_id BIGINT REFERENCES second_categories(id),
    third_category_id BIGINT REFERENCES third_categories(id),
    price_zone_guideline_id BIGINT REFERENCES price_zone_guidelines(id),
    PRIMARY KEY (brand_id, slot_number),
    CHECK ((second_category_id IS NOT NULL) <> (third_category_id IS NOT NULL)),
    UNIQUE (brand_id, second_category_id),
    UNIQUE (brand_id, third_category_id),
    UNIQUE (brand_id, price_zone_guideline_id)
);
CREATE INDEX idx_brand_screening_genre ON brand_price_screenings(first_category_id);
CREATE INDEX idx_brand_screening_second ON brand_price_screening_products(second_category_id);
CREATE INDEX idx_brand_screening_third ON brand_price_screening_products(third_category_id);
CREATE INDEX idx_brand_screening_guideline ON brand_price_screening_products(price_zone_guideline_id);

CREATE TABLE shop_price_screenings (
    shop_id BIGINT PRIMARY KEY REFERENCES shops(id) ON DELETE CASCADE,
    first_category_id BIGINT NOT NULL REFERENCES first_categories(id),
    saved_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE shop_price_screening_products (
    shop_id BIGINT NOT NULL REFERENCES shop_price_screenings(shop_id) ON DELETE CASCADE,
    slot_number SMALLINT NOT NULL CHECK (slot_number BETWEEN 1 AND 3),
    second_category_id BIGINT REFERENCES second_categories(id),
    third_category_id BIGINT REFERENCES third_categories(id),
    price_zone_guideline_id BIGINT REFERENCES price_zone_guidelines(id),
    PRIMARY KEY (shop_id, slot_number),
    CHECK ((second_category_id IS NOT NULL) <> (third_category_id IS NOT NULL)),
    UNIQUE (shop_id, second_category_id),
    UNIQUE (shop_id, third_category_id),
    UNIQUE (shop_id, price_zone_guideline_id)
);
CREATE INDEX idx_shop_screening_genre ON shop_price_screenings(first_category_id);
CREATE INDEX idx_shop_screening_second ON shop_price_screening_products(second_category_id);
CREATE INDEX idx_shop_screening_third ON shop_price_screening_products(third_category_id);
CREATE INDEX idx_shop_screening_guideline ON shop_price_screening_products(price_zone_guideline_id);

COMMIT;
