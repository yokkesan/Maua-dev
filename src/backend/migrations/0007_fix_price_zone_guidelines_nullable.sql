BEGIN;

ALTER TABLE price_zone_guidelines
    ALTER COLUMN second_id DROP NOT NULL,
    ALTER COLUMN third_id DROP NOT NULL;

ALTER TABLE price_zone_guidelines
    ADD CONSTRAINT chk_price_zone_guidelines_category
    CHECK (
        (second_id IS NOT NULL AND third_id IS NULL)
        OR
        (second_id IS NULL AND third_id IS NOT NULL)
    );

COMMIT;