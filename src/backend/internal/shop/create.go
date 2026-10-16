package shop

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type CreateShopInput map[string]json.RawMessage

func (s *Service) Create(
	ctx context.Context,
	companyID int64,
	input CreateShopInput,
) (map[string]any, error) {
	if companyID <= 0 || input == nil {
		return nil, ErrInvalidShopInput
	}

	values := map[string]any{
		"phonetic": "",
		"tel":      "",
		"email":    "",
	}

	// POST APIではショップ名を必須とする。
	nameRaw, ok := input["name"]
	if !ok {
		return nil, ErrInvalidShopInput
	}

	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil {
		return nil, ErrInvalidShopInput
	}
	if strings.TrimSpace(name) == "" ||
		utf8.RuneCountInString(name) > 255 {
		return nil, ErrInvalidShopInput
	}
	values["name"] = name

	planRaw, ok := input["registration_plan"]
	if !ok {
		return nil, ErrInvalidShopInput
	}

	var plan int64
	if err := json.Unmarshal(planRaw, &plan); err != nil {
		return nil, ErrInvalidShopInput
	}
	if plan != 10 && plan != 11 {
		return nil, ErrInvalidShopInput
	}
	values["registration_plan"] = plan

	basicLengths := map[string]int{
		"phonetic": 255,
		"tel":      32,
		"email":    254,
	}

	for key, maxLength := range basicLengths {
		raw, exists := input[key]
		if !exists {
			continue
		}

		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, ErrInvalidShopInput
		}
		if utf8.RuneCountInString(value) > maxLength {
			return nil, ErrInvalidShopInput
		}

		values[key] = value
	}

	for key, raw := range input {
		if key == "name" || key == "registration_plan" {
			continue
		}
		if _, basic := basicLengths[key]; basic {
			continue
		}

		if key == "category_ids" {
			if strings.TrimSpace(string(raw)) == "null" {
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
				if _, duplicate := seen[id]; duplicate {
					return nil, ErrInvalidShopInput
				}
				seen[id] = struct{}{}
			}

			values[key] = ids
			continue
		}

		field, allowed := shopUpdateFields[key]
		if !allowed {
			return nil, ErrInvalidShopInput
		}

		if strings.TrimSpace(string(raw)) == "null" {
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

	return s.repository.Create(ctx, companyID, values)
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	companyID int64,
	values map[string]any,
) (map[string]any, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin shop create: %w", err)
	}
	defer tx.Rollback(ctx)

	columns := []string{"company_id"}
	args := []any{companyID}
	placeholders := []string{"$1"}

	keys := make([]string, 0)
	for key := range values {
		switch key {
		case "name", "phonetic", "tel", "email", "registration_plan":
			keys = append(keys, key)
		case "category_ids":
			// 関連テーブルに登録するのでINSERTから除外
		default:
			if _, ok := shopUpdateFields[key]; !ok {
				return nil, ErrInvalidShopInput
			}
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		columns = append(columns, key)
		args = append(args, values[key])
		placeholders = append(
			placeholders,
			fmt.Sprintf("$%d", len(args)),
		)
	}

	query := fmt.Sprintf(
		"INSERT INTO shops (%s) VALUES (%s) RETURNING id",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	var shopID int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&shopID); err != nil {
		if isShopInputConstraintError(err) {
			return nil, ErrInvalidShopInput
		}
		return nil, fmt.Errorf("insert shop: %w", err)
	}

	if rawIDs, exists := values["category_ids"]; exists {
		ids, ok := rawIDs.([]int64)
		if !ok {
			return nil, ErrInvalidShopInput
		}

		for _, categoryID := range ids {
			_, err := tx.Exec(
				ctx,
				`INSERT INTO shop_categories (shop_id, category_id)
				 VALUES ($1, $2)`,
				shopID,
				categoryID,
			)
			if err != nil {
				if isShopInputConstraintError(err) {
					return nil, ErrInvalidShopInput
				}
				return nil, fmt.Errorf("insert shop category: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit shop create: %w", err)
	}

	return r.FindDetailByID(ctx, shopID, companyID)
}
