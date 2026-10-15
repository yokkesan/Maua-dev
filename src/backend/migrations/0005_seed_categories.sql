BEGIN;

-- =========================================================
-- 既存データの確認
-- =========================================================

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM first_categories)
        OR EXISTS (SELECT 1 FROM second_categories)
        OR EXISTS (SELECT 1 FROM third_categories)
    THEN
        RAISE EXCEPTION 'Category master tables are not empty';
    END IF;
END
$$;

-- =========================================================
-- 第1階層：4件
-- =========================================================

INSERT INTO first_categories (id, name)
VALUES
    (1, 'Apparel'),
    (2, 'Life Style'),
    (3, 'Food'),
    (4, 'Beauty&Health');


-- =========================================================
-- 第2階層：43件
-- =========================================================

INSERT INTO second_categories (id, first_category_id, name)
VALUES
    -- Apparel
    (1, 1, 'Mens'),
    (2, 1, 'Ladies'),
    (3, 1, 'Unisex'),
    (4, 1, 'Organic'),
    (5, 1, 'Sustainable'),
    (6, 1, 'Sport'),
    (7, 1, 'アパレル雑貨'),
    (8, 1, 'その他'),

    -- Life Style
    (9, 2, 'インテリア'),
    (10, 2, '家具'),
    (11, 2, 'ファブリック'),
    (12, 2, 'アート・陶芸・ピクチャー'),
    (13, 2, 'キッチン'),
    (14, 2, 'ガーデン'),
    (15, 2, '雑貨・ステーショナリー'),
    (16, 2, 'PC・スマホグッズ・ガジェット'),
    (17, 2, '音楽・本'),
    (18, 2, 'キッズ・マタニティ'),
    (19, 2, 'ペット'),
    (20, 2, 'その他'),

    -- Food
    (21, 3, '野菜'),
    (22, 3, 'フルーツ'),
    (23, 3, 'スイーツ'),
    (24, 3, 'コーヒー'),
    (25, 3, 'ハーブティ・お茶'),
    (26, 3, 'ジュース'),
    (27, 3, 'アルコール'),
    (28, 3, '麺・パスタ'),
    (29, 3, 'パン'),
    (30, 3, 'お米・お餅'),
    (31, 3, '調味料・スパイス'),
    (32, 3, 'レトルト'),

    -- Beauty&Health
    (33, 4, 'スキンケア'),
    (34, 4, 'メイク'),
    (35, 4, 'ネイルグッズ'),
    (36, 4, '香水・フレグランス'),
    (37, 4, 'ヘアケア'),
    (38, 4, 'ダイエット'),
    (39, 4, 'サプリ'),
    (40, 4, 'ボディーメイク・ウェア・下着'),
    (41, 4, 'バスグッズ'),
    (42, 4, 'メンズコスメ'),
    (43, 4, '美容器具');


-- =========================================================
-- 第3階層：98件
-- Apparelの7カテゴリーそれぞれに14種類を登録
-- =========================================================

WITH third_names (position, name) AS (
    VALUES
        (1, 'ジャケット・アウター'),
        (2, 'スーツ'),
        (3, 'ワンピース'),
        (4, 'スカート'),
        (5, 'パンツ'),
        (6, 'シャツ'),
        (7, 'ニット・スウェット'),
        (8, 'Tシャツ・カットソー'),
        (9, 'バック・小物'),
        (10, '帽子'),
        (11, 'シューズ'),
        (12, 'ジュエリー'),
        (13, 'アクセサリー・時計'),
        (14, '下着・ルームウエア')
)

INSERT INTO third_categories (
    id,
    second_category_id,
    name
)
SELECT
    (s.id - 1) * 14 + t.position,
    s.id,
    t.name
FROM second_categories s
CROSS JOIN third_names t
WHERE
    s.first_category_id = 1
    AND s.name <> 'その他'
ORDER BY
    s.id,
    t.position;


-- =========================================================
-- BIGSERIALのシーケンスを更新
-- =========================================================

SELECT setval(
    pg_get_serial_sequence('first_categories', 'id'),
    (SELECT MAX(id) FROM first_categories)
);

SELECT setval(
    pg_get_serial_sequence('second_categories', 'id'),
    (SELECT MAX(id) FROM second_categories)
);

SELECT setval(
    pg_get_serial_sequence('third_categories', 'id'),
    (SELECT MAX(id) FROM third_categories)
);

COMMIT;