BEGIN;

-- ブランドのメイン画像
CREATE TABLE brand_images (
    id BIGSERIAL PRIMARY KEY,
    brand_id BIGINT NOT NULL
        REFERENCES brands(id) ON DELETE CASCADE,
    slot_number SMALLINT NOT NULL
        CHECK (slot_number BETWEEN 1 AND 2),
    storage_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (brand_id, slot_number)
);

-- ショップのメイン画像
CREATE TABLE shop_images (
    id BIGSERIAL PRIMARY KEY,
    shop_id BIGINT NOT NULL
        REFERENCES shops(id) ON DELETE CASCADE,
    slot_number SMALLINT NOT NULL
        CHECK (slot_number BETWEEN 1 AND 2),
    storage_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (shop_id, slot_number)
);

COMMIT;