package brand

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrInvalidBrandInput = errors.New("invalid brand input")

type UpdateBrandInput map[string]json.RawMessage

type brandUpdateField struct {
	MaxLength int
	Min       *int64
	Max       *int64
}

func int64Ptr(n int64) *int64 {
	return &n
}

// brandsテーブルの更新可能カラム。
// company_id、approval、statusなどは含めない。
var brandUpdateFields = map[string]brandUpdateField{
	"concept":                 {},
	"top_news":                {},
	"other_brand":             {},
	"target_min":              {MaxLength: 20},
	"target_max":              {MaxLength: 20},
	"web_url":                 {},
	"net_shop":                {Min: int64Ptr(0), Max: int64Ptr(4)},
	"contact_person_name":     {MaxLength: 255},
	"contact_person_phonetic": {MaxLength: 255},
	"contact_person_tel":      {MaxLength: 50},
	"facebook":                {},
	"twitter":                 {},
	"instagram":               {},
	"line":                    {},
	"price_min":               {MaxLength: 50},
	"price_max":               {MaxLength: 50},
	"base":                    {},
	"rate_dm_form":            {Min: int64Ptr(0), Max: int64Ptr(5)},
	"rate_min":                {MaxLength: 50},
	"rate_max":                {MaxLength: 50},
	"box_price_min":           {MaxLength: 50},
	"box_price_max":           {MaxLength: 50},
	"one_gram":                {MaxLength: 50},
	"gram_price_min":          {MaxLength: 50},
	"gram_price_max":          {MaxLength: 50},
	"price_free_text":         {},
	"lot_free_text":           {},
	"price_zone_id":           {Min: int64Ptr(1)},
}

// 関連テーブルの更新可能項目。
// テーブル名とカラム名はサーバー側で固定する。
type brandRelation struct {
	Table    string
	IDColumn string
}

var brandUpdateRelations = map[string]brandRelation{
	"category_ids": {
		Table:    "brand_categories",
		IDColumn: "category_id",
	},
	"city_ids": {
		Table:    "brand_areas",
		IDColumn: "city_id",
	},
	"sales_recruitment_ids": {
		Table:    "brand_sales_recruitments",
		IDColumn: "sales_recruitment_id",
	},
	"sales_style_ids": {
		Table:    "brand_sales_styles",
		IDColumn: "sales_style_id",
	},
	"order_ids": {
		Table:    "brand_orders",
		IDColumn: "order_id",
	},
	"pay_rule_ids": {
		Table:    "brand_pay_rules",
		IDColumn: "pay_rule_id",
	},
}

// UpdateByID は更新データを検証してRepositoryへ渡す。
func (s *Service) UpdateByID(
	ctx context.Context,
	brandID int64,
	companyID int64,
	input UpdateBrandInput,
) (map[string]any, error) {

	if brandID <= 0 || companyID <= 0 {
		return nil, ErrInvalidBrandInput
	}

	if input == nil {
		return nil, ErrInvalidBrandInput
	}

	values := make(map[string]any, len(input))

	for key, raw := range input {
		raw = json.RawMessage(
			strings.TrimSpace(string(raw)),
		)

		// 関連テーブルの更新
		if _, ok := brandUpdateRelations[key]; ok {
			// 関連情報はnullではなく[]で解除する。
			if string(raw) == "null" {
				return nil, ErrInvalidBrandInput
			}

			var ids []int64

			if err := json.Unmarshal(raw, &ids); err != nil {
				return nil, ErrInvalidBrandInput
			}

			if ids == nil || len(ids) > 1000 {
				return nil, ErrInvalidBrandInput
			}

			seen := make(map[int64]struct{}, len(ids))

			for _, id := range ids {
				if id <= 0 {
					return nil, ErrInvalidBrandInput
				}

				if _, exists := seen[id]; exists {
					return nil, ErrInvalidBrandInput
				}

				seen[id] = struct{}{}
			}

			values[key] = ids
			continue
		}

		// brandsテーブルの更新
		field, ok := brandUpdateFields[key]
		if !ok {
			return nil, ErrInvalidBrandInput
		}

		// 明示的なnullはDBのNULLに更新する。
		if string(raw) == "null" {
			values[key] = nil
			continue
		}

		// 数値項目
		if field.Min != nil || field.Max != nil {
			var number int64

			if err := json.Unmarshal(raw, &number); err != nil {
				return nil, ErrInvalidBrandInput
			}

			if field.Min != nil && number < *field.Min {
				return nil, ErrInvalidBrandInput
			}

			if field.Max != nil && number > *field.Max {
				return nil, ErrInvalidBrandInput
			}

			values[key] = number
			continue
		}

		// 文字列項目
		var value string

		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, ErrInvalidBrandInput
		}

		if field.MaxLength > 0 &&
			utf8.RuneCountInString(value) > field.MaxLength {
			return nil, ErrInvalidBrandInput
		}

		values[key] = value
	}

	return s.repository.UpdateByID(
		ctx,
		brandID,
		companyID,
		values,
	)
}

// UpdateByID は基本情報と関連テーブルを一括更新する。
func (r *PostgresRepository) UpdateByID(
	ctx context.Context,
	brandID int64,
	companyID int64,
	values map[string]any,
) (map[string]any, error) {

	// Repository側でも更新可能項目を制限する。
	for key := range values {
		if _, ok := brandUpdateFields[key]; ok {
			continue
		}

		if _, ok := brandUpdateRelations[key]; ok {
			continue
		}

		return nil, ErrInvalidBrandInput
	}

	tx, err := r.db.BeginTx(
		ctx,
		pgx.TxOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("begin brand update: %w", err)
	}
	defer tx.Rollback(ctx)

	// 所有権を確認し、対象ブランドをロックする。
	var existingID int64

	err = tx.QueryRow(
		ctx,
		`
			SELECT id
			FROM brands
			WHERE id = $1
			  AND company_id = $2
			FOR UPDATE
		`,
		brandID,
		companyID,
	).Scan(&existingID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}

		return nil, fmt.Errorf("lock brand: %w", err)
	}

	// 回答登録と同じブランドロックの下で、更新後の整合性を確認する。
	if _, updating := values["price_zone_id"]; updating {
		var hasAnswers bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM brand_price_zone_guidelines WHERE brand_id=$1
		)`, brandID).Scan(&hasAnswers); err != nil {
			return nil, fmt.Errorf("check brand price screening answers: %w", err)
		}
		if hasAnswers {
			return nil, ErrInvalidBrandInput
		}
	}

	if value, updating := values["category_ids"]; updating {
		ids, ok := value.([]int64)
		if !ok {
			return nil, ErrInvalidBrandInput
		}
		var losesCategory bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM brand_price_zone_guidelines bg
			JOIN price_zone_guidelines g ON g.id=bg.price_zone_guideline_id
			LEFT JOIN third_categories th ON th.id=g.third_id
			LEFT JOIN second_categories s ON s.id=COALESCE(g.second_id, th.second_category_id)
			WHERE bg.brand_id=$1 AND NOT EXISTS (
				SELECT 1 FROM categories c
				WHERE c.id=ANY($2::bigint[])
				  AND c.first_id=s.first_category_id AND c.second_id=s.id
				  AND ((g.third_id IS NOT NULL AND c.third_id=g.third_id)
				       OR (g.second_id IS NOT NULL AND c.third_id IS NULL))
			)
		)`, brandID, ids).Scan(&losesCategory); err != nil {
			return nil, fmt.Errorf("check brand price screening categories: %w", err)
		}
		if losesCategory {
			return nil, ErrInvalidBrandInput
		}
	}

	// -----------------------------------------------------
	// 基本情報
	// -----------------------------------------------------
	columns := make([]string, 0)

	for key := range values {
		if _, ok := brandUpdateFields[key]; ok {
			columns = append(columns, key)
		}
	}

	sort.Strings(columns)

	if len(columns) > 0 {
		setParts := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns)+2)

		args = append(args, brandID, companyID)

		for i, column := range columns {
			setParts = append(
				setParts,
				fmt.Sprintf("%s = $%d", column, i+3),
			)

			args = append(args, values[column])
		}

		query := fmt.Sprintf(
			`
				UPDATE brands
				SET %s
				WHERE id = $1
				  AND company_id = $2
			`,
			strings.Join(setParts, ", "),
		)

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			if isBrandInputConstraintError(err) {
				return nil, ErrInvalidBrandInput
			}

			return nil, fmt.Errorf(
				"update brand fields: %w",
				err,
			)
		}
	}

	// -----------------------------------------------------
	// 関連テーブル
	// -----------------------------------------------------
	relationKeys := make([]string, 0)

	for key := range values {
		if _, ok := brandUpdateRelations[key]; ok {
			relationKeys = append(relationKeys, key)
		}
	}

	sort.Strings(relationKeys)

	for _, key := range relationKeys {
		relation := brandUpdateRelations[key]

		ids, ok := values[key].([]int64)
		if !ok {
			return nil, ErrInvalidBrandInput
		}

		// 指定された関連情報のみ既存データを削除。
		deleteQuery := fmt.Sprintf(
			"DELETE FROM %s WHERE brand_id = $1",
			relation.Table,
		)

		if _, err := tx.Exec(
			ctx,
			deleteQuery,
			brandID,
		); err != nil {
			return nil, fmt.Errorf(
				"delete brand relation %s: %w",
				key,
				err,
			)
		}

		// []なら削除だけで完了する。
		insertQuery := fmt.Sprintf(
			`
				INSERT INTO %s (
					brand_id,
					%s
				)
				VALUES ($1, $2)
			`,
			relation.Table,
			relation.IDColumn,
		)

		for _, id := range ids {
			if _, err := tx.Exec(
				ctx,
				insertQuery,
				brandID,
				id,
			); err != nil {
				if isBrandInputConstraintError(err) {
					return nil, ErrInvalidBrandInput
				}

				return nil, fmt.Errorf(
					"insert brand relation %s: %w",
					key,
					err,
				)
			}
		}
	}

	// すべて成功した場合だけコミット。
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf(
			"commit brand update: %w",
			err,
		)
	}

	// 更新済みの情報を取得する。
	return r.FindDetailByID(
		ctx,
		brandID,
		companyID,
	)
}

// PostgreSQLの入力・参照整合性エラーを判定する。
func isBrandInputConstraintError(err error) bool {
	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case "23503", // foreign_key_violation
		"23505", // unique_violation
		"23514", // check_violation
		"22001", // string_data_right_truncation
		"22P02": // invalid_text_representation
		return true
	}

	return false
}
