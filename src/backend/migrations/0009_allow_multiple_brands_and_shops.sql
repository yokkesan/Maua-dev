BEGIN;

-- 1会社に複数ブランドを許可
ALTER TABLE brands
    DROP CONSTRAINT IF EXISTS brands_company_id_unique;

-- 1会社に複数ショップを許可
ALTER TABLE shops
    DROP CONSTRAINT IF EXISTS shops_company_id_unique;

-- 会社ID検索用の通常インデックス
CREATE INDEX IF NOT EXISTS idx_brands_company_id
    ON brands(company_id);

CREATE INDEX IF NOT EXISTS idx_shops_company_id
    ON shops(company_id);

COMMIT;