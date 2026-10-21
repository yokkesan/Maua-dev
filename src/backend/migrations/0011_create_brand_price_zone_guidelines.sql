BEGIN;

CREATE TABLE brand_price_zone_guidelines (
    brand_id BIGINT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    price_zone_guideline_id BIGINT NOT NULL REFERENCES price_zone_guidelines(id),
    CONSTRAINT brand_price_zone_guidelines_unique
        UNIQUE (brand_id, price_zone_guideline_id)
);

-- UNIQUEの先頭列でブランド別取得を、こちらでガイドラインの参照確認を支える。
CREATE INDEX idx_brand_price_zone_guidelines_guideline_id
    ON brand_price_zone_guidelines(price_zone_guideline_id);

COMMIT;
