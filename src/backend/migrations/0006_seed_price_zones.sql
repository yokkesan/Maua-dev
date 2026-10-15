BEGIN;


-- =========================================================
-- 1. 既存データの確認
-- =========================================================

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM price_zones)
        OR EXISTS (SELECT 1 FROM price_zone_guidelines)
    THEN
        RAISE EXCEPTION 'Price zone master tables are not empty';
    END IF;

    IF (SELECT COUNT(*) FROM second_categories) <> 43
        OR (SELECT COUNT(*) FROM third_categories) <> 98
    THEN
        RAISE EXCEPTION 'Category master data is incomplete';
    END IF;
END
$$;


-- =========================================================
-- 2. 価格帯：3件
-- =========================================================

INSERT INTO price_zones (id, name)
VALUES
    (1, 'Standard'),
    (2, 'Highclass'),
    (3, 'Luxury');

WITH price_definitions (
    category_name,
    rank3,
    rank2,
    rank1
) AS (
    VALUES
        (
            'ジャケット・アウター',
            '80,000以上',
            '30,000以上80,000以下',
            '30,000以下'
        ),
        (
            'スーツ',
            '200,000以上',
            '80,000以上200,000以下',
            '80,000以下'
        ),
        (
            'ワンピース',
            '80,000以上',
            '30,000以上,80,000以下',
            '30,000以下'
        ),
        (
            'スカート',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            'パンツ',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            'シャツ',
            '50,000以上',
            '50,000以下',
            '20,000以下'
        ),
        (
            'ニット・スウェット',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            'Tシャツ・カットソー',
            '30,000以上',
            '10,000以上、30,000以下',
            '10,000以下'
        ),
        (
            'バック・小物',
            '60,000以上',
            '30,000以上,60,000以下',
            '30,000以下'
        ),
        (
            '帽子',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            'シューズ',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            'ジュエリー',
            '200,000以上',
            '50,000以上,200,000以下',
            '50,000以下'
        ),
        (
            'アクセサリー・時計',
            '60,000以上',
            '20,000以上,60,000以下',
            '20,000以下'
        ),
        (
            '下着・ルームウエア',
            '20,000以上',
            '5,000以上、20,000以下',
            '5,000以下'
        )
)

INSERT INTO price_zone_guidelines (
    second_id,
    third_id,
    price_rank,
    price_range
)
SELECT
    NULL,
    third.id,
    rank_data.price_rank,
    rank_data.price_range

FROM third_categories AS third

INNER JOIN second_categories AS second
    ON second.id = third.second_category_id

INNER JOIN price_definitions AS definitions
    ON definitions.category_name = third.name

CROSS JOIN LATERAL (
    VALUES
        (3, definitions.rank3),
        (2, definitions.rank2),
        (1, definitions.rank1)
) AS rank_data(price_rank, price_range)

WHERE
    second.first_category_id = 1
    AND second.name <> 'その他';


-- =========================================================
-- 4. 第2階層カテゴリー：108件
--
-- second_id 8〜43
-- 36カテゴリー × 3ランク
-- =========================================================

WITH price_definitions (
    second_id,
    rank3,
    rank2,
    rank1
) AS (
    VALUES
        -- Apparel：その他
        (8, '20,000以上', '5,000以上、20,000以下', '5,000以下'),

        -- Life Style
        (9, '100,000以上', '40,000以上,100,000以下', '40,000以下'),
        (10, '300,000以上', '100,000以上,300,000以下', '100,000以下'),
        (11, '100,000以上', '20,000以上,100,000以下', '20,000以下'),
        (12, '100,000以上', '10,000以上,100,000以下', '10,000以下'),
        (13, '100,000以上', '10,000以上,100,000以下', '10,000以下'),
        (14, '15,000以上', '5,000以上,15,000以下', '5,000以下'),
        (15, '20,000以上', '3,000以上,20,000以下', '3,000以下'),
        (16, '20,000以上', '3,000以上,20,000以下', '3,000以下'),
        (17, '10,000以上', '2,000以上,10,000以下', '2,000以下'),
        (18, '10,000以上', '3,000以上,10,000以下', '3,000以下'),
        (19, '10,000以上', '3,000以上,10,000以下', '3,000以下'),
        (20, '10,000以上', '3,000以上,10,000以下', '3,000以下'),

        -- Food
        (21, '3,000以上', '1,000以上,3,000以下', '1,000以下'),
        (22, '5,000以上', '1,000以上,5,000以下', '1,000以下'),
        (23, '5,000以上', '2,000以上,5,000以下', '2,000以下'),
        (24, '3,000以上', '1,000以上3,000以下', '1,000以下'),
        (25, '10,000以上', '1,000以上10,000以下', '1,000以下'),
        (26, '5,000以上', '2,000以上5,000以下', '2,000以下'),
        (27, '5,000以上', '2,000以上5,000以下', '2,000以下'),
        (28, '5,000以上', '1,000以上5,000以下', '1,000以下'),
        (29, '5,000以上', '2,000以上5,000以下', '2,000以下'),
        (30, '10,000以上', '2,000以上5,000以下', '2,000以下'),
        (31, '3,000以上', '1,000以上3,000以下', '1,000以下'),
        (32, '5,000以上', '1,000以上5,000以下', '1,000以下'),

        -- Beauty&Health
        (33, '30,000以上', '6,000以上、30,000以下', '6,000以下'),
        (34, '15,000以上', '5,000以上、15,000以下', '5,000以下'),
        (35, '8,000以上', '3,000以上、8,000以下', '3,000以下'),
        (36, '30,000以上', '5,000以上、30,000以下', '5,000以下'),
        (37, '10,000以上', '5,000以上、10,000以下', '5,000以下'),
        (38, '15,000以上', '5,000以上15,000以下', '5,000以下'),
        (39, '15,000以上', '5,000以上15,000以下', '5,000以下'),
        (40, '50,000以上', '5,000以上50,000以下', '5,000以下'),
        (41, '8,000以上', '3,000以上、8,000以下', '3,000以下'),
        (42, '30,000以上', '6,000以上、30,000以下', '6,000以下'),
        (43, '100,000以上', '50,000以上100,000以下', '50,000以下')
)

INSERT INTO price_zone_guidelines (
    second_id,
    third_id,
    price_rank,
    price_range
)
SELECT
    definitions.second_id,
    NULL,
    rank_data.price_rank,
    rank_data.price_range

FROM price_definitions AS definitions

INNER JOIN second_categories AS second
    ON second.id = definitions.second_id

CROSS JOIN LATERAL (
    VALUES
        (3, definitions.rank3),
        (2, definitions.rank2),
        (1, definitions.rank1)
) AS rank_data(price_rank, price_range);


-- =========================================================
-- 5. 登録件数の検証
-- =========================================================

DO $$
DECLARE
    zone_count INTEGER;
    guideline_count INTEGER;
    third_guideline_count INTEGER;
    second_guideline_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO zone_count
    FROM price_zones;

    SELECT COUNT(*)
    INTO guideline_count
    FROM price_zone_guidelines;

    SELECT COUNT(*)
    INTO third_guideline_count
    FROM price_zone_guidelines
    WHERE third_id IS NOT NULL;

    SELECT COUNT(*)
    INTO second_guideline_count
    FROM price_zone_guidelines
    WHERE second_id IS NOT NULL;

    IF zone_count <> 3
        OR guideline_count <> 402
        OR third_guideline_count <> 294
        OR second_guideline_count <> 108
    THEN
        RAISE EXCEPTION
            'Invalid price master counts: zones=%, total=%, third=%, second=%',
            zone_count,
            guideline_count,
            third_guideline_count,
            second_guideline_count;
    END IF;
END
$$;


-- =========================================================
-- 6. price_zonesのシーケンス更新
-- =========================================================

SELECT setval(
    pg_get_serial_sequence('price_zones', 'id'),
    (SELECT MAX(id) FROM price_zones)
);

COMMIT;