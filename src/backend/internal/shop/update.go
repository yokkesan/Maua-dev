package shop

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

var ErrInvalidShopInput = errors.New("invalid shop input")

// キーなし：更新しない
// null：DBのNULLに更新
// 値あり：指定値に更新
type UpdateShopInput map[string]json.RawMessage

type shopUpdateField struct {
	MaxLength int
	Min       *int64
	Max       *int64
}

func shopInt64Ptr(value int64) *int64 {
	return &value
}

// ショップの更新可能項目
var shopUpdateFields = map[string]shopUpdateField{
	"concept":                 {},
	"target_min":              {MaxLength: 20},
	"target_max":              {MaxLength: 20},
	"web_url":                 {},
	"net_shop":                {Min: shopInt64Ptr(0), Max: shopInt64Ptr(4)},
	"contact_person_name":     {MaxLength: 255},
	"contact_person_phonetic": {MaxLength: 255},
	"contact_person_tel":      {MaxLength: 50},
	"facebook":                {},
	"twitter":                 {},
	"instagram":               {},
	"line":                    {},
	"address":                 {},
	"handling_brand":          {},
	"price_zone_id":           {Min: shopInt64Ptr(1)},
}

// UpdateByID は入力値の検証後、ショップを更新する。
func (s *Service) UpdateByID(
	ctx context.Context,
	shopID int64,
	companyID int64,
	input UpdateShopInput,
) (map[string]any, error) {
	if shopID <= 0 || companyID <= 0 || input == nil {
		return nil, ErrInvalidShopInput
	}

	values := make(map[string]any, len(input))

	for key, raw := range input {
		raw = json.RawMessage(strings.TrimSpace(string(raw)))

		// カテゴリーは配列で差し替える
		if key == "category_ids" {
			if string(raw) == "null" {
				return nil, ErrInvalidShopInput
			}

			var ids []int64

			if err := json.Unmarshal(raw, &ids); err != nil {
				return nil, ErrInvalidShopInput
			}

			if ids == nil || len(ids) > 1000 {
				return nil, ErrInvalidShopInput
			}

			seen := make(map[int64]struct{}, len(ids))

			for _, id := range ids {
				if id <= 0 {
					return nil, ErrInvalidShopInput
				}

				if _, exists := seen[id]; exists {
					return nil, ErrInvalidShopInput
				}

				seen[id] = struct{}{}
			}

			values[key] = ids
			continue
		}

		field, ok := shopUpdateFields[key]
		if !ok {
			return nil, ErrInvalidShopInput
		}

		if string(raw) == "null" {
			values[key] = nil
			continue
		}

		if field.Min != nil || field.Max != nil {
			var number int64

			if err := json.Unmarshal(raw, &number); err != nil {
				return nil, ErrInvalidShopInput
			}

			if field.Min != nil && number < *field.Min {
				return nil, ErrInvalidShopInput
			}

			if field.Max != nil && number > *field.Max {
				return nil, ErrInvalidShopInput
			}

			values[key] = number
			continue
		}

		var value string

		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, ErrInvalidShopInput
		}

		if field.MaxLength > 0 &&
			utf8.RuneCountInString(value) > field.MaxLength {
			return nil, ErrInvalidShopInput
		}

		values[key] = value
	}

	return s.repository.UpdateByID(
		ctx,
		shopID,
		companyID,
		values,
	)
}

// UpdateByID はショップ基本情報とカテゴリーを一括更新する。
func (r *PostgresRepository) UpdateByID(
	ctx context.Context,
	shopID int64,
	companyID int64,
	values map[string]any,
) (map[string]any, error) {
	// SQLに使用するカラムをホワイトリストで制限
	for key := range values {
		if key == "category_ids" {
			continue
		}

		if _, ok := shopUpdateFields[key]; !ok {
			return nil, ErrInvalidShopInput
		}
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin shop update: %w", err)
	}
	defer tx.Rollback(ctx)

	// 自社ショップのみ更新可能。行ロックで同時更新を制御。
	var existingID int64

	err = tx.QueryRow(
		ctx,
		`
			SELECT id
			FROM shops
			WHERE id = $1
			  AND company_id = $2
			FOR UPDATE
		`,
		shopID,
		companyID,
	).Scan(&existingID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShopNotFound
		}
		return nil, fmt.Errorf("lock shop: %w", err)
	}

	// 基本情報の更新
	columns := make([]string, 0, len(values))

	for key := range values {
		if _, ok := shopUpdateFields[key]; ok {
			columns = append(columns, key)
		}
	}

	sort.Strings(columns)

	if len(columns) > 0 {
		setParts := make([]string, 0, len(columns))
		args := make([]any, 0, len(columns)+2)

		args = append(args, shopID, companyID)

		for i, column := range columns {
			setParts = append(
				setParts,
				fmt.Sprintf("%s = $%d", column, i+3),
			)
			args = append(args, values[column])
		}

		query := fmt.Sprintf(
			`
				UPDATE shops
				SET %s
				WHERE id = $1
				  AND company_id = $2
			`,
			strings.Join(setParts, ", "),
		)

		if _, err := tx.Exec(ctx, query, args...); err != nil {
			if isShopInputConstraintError(err) {
				return nil, ErrInvalidShopInput
			}
			return nil, fmt.Errorf("update shop fields: %w", err)
		}
	}

	// カテゴリーが指定された場合だけ差し替え
	if rawIDs, exists := values["category_ids"]; exists {
		ids, ok := rawIDs.([]int64)
		if !ok {
			return nil, ErrInvalidShopInput
		}

		if _, err := tx.Exec(
			ctx,
			"DELETE FROM shop_categories WHERE shop_id = $1",
			shopID,
		); err != nil {
			return nil, fmt.Errorf("delete shop categories: %w", err)
		}

		for _, categoryID := range ids {
			if _, err := tx.Exec(
				ctx,
				`
					INSERT INTO shop_categories (
						shop_id,
						category_id
					)
					VALUES ($1, $2)
				`,
				shopID,
				categoryID,
			); err != nil {
				if isShopInputConstraintError(err) {
					return nil, ErrInvalidShopInput
				}
				return nil, fmt.Errorf("insert shop category: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit shop update: %w", err)
	}

	return r.FindDetailByID(ctx, shopID, companyID)
}

func isShopInputConstraintError(err error) bool {
	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case "23503", "23505", "23514", "22001", "22P02":
		return true
	default:
		return false
	}
}
