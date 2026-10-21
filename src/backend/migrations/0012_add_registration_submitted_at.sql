BEGIN;

ALTER TABLE brands ADD COLUMN submitted_at TIMESTAMPTZ NULL;
ALTER TABLE shops ADD COLUMN submitted_at TIMESTAMPTZ NULL;

CREATE INDEX idx_brands_pending_submission ON brands (submitted_at, id)
    WHERE submitted_at IS NOT NULL AND approval = 0;
CREATE INDEX idx_shops_pending_submission ON shops (submitted_at, id)
    WHERE submitted_at IS NOT NULL AND approval = 0;

COMMIT;
