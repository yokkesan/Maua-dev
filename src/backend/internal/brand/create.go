package brand

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type CreateBrandInput map[string]json.RawMessage

func (s *Service) Create(
	ctx context.Context,
	companyID int64,
	input CreateBrandInput,
) (map[string]any, error) {
	if companyID <= 0 || input == nil {
		return nil, ErrInvalidBrandInput
	}

	nameRaw, exists := input["name"]
	if !exists {
		return nil, ErrInvalidBrandInput
	}

	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil {
		return nil, ErrInvalidBrandInput
	}

	if strings.TrimSpace(name) == "" ||
		utf8.RuneCountInString(name) > 255 {
		return nil, ErrInvalidBrandInput
	}

	values := map[string]any{
		"name":     name,
		"phonetic": "",
		"tel":      "",
		"email":    "",
	}

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
			return nil, ErrInvalidBrandInput
		}

		if utf8.RuneCountInString(value) > maxLength {
			return nil, ErrInvalidBrandInput
		}

		values[key] = value
	}

	for key, raw := range input {
		if key == "name" {
			continue
		}
		if _, basic := basicLengths[key]; basic {
			continue
		}

		if _, relation := brandUpdateRelations[key]; relation {
			if strings.TrimSpace(string(raw)) == "null" {
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
				if _, duplicate := seen[id]; duplicate {
					return nil, ErrInvalidBrandInput
				}
				seen[id] = struct{}{}
			}

			values[key] = ids
			continue
		}

		field, allowed := brandUpdateFields[key]
		if !allowed {
			return nil, ErrInvalidBrandInput
		}

		if strings.TrimSpace(string(raw)) == "null" {
			values[key] = nil
			continue
		}

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

	return s.repository.Create(ctx, companyID, values)
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	companyID int64,
	values map[string]any,
) (map[string]any, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin brand create: %w", err)
	}
	defer tx.Rollback(ctx)

	columns := []string{"company_id"}
	args := []any{companyID}
	placeholders := []string{"$1"}

	fieldKeys := make([]string, 0)

	for key := range values {
		switch key {
		case "name", "phonetic", "tel", "email":
			fieldKeys = append(fieldKeys, key)
		default:
			if _, ok := brandUpdateFields[key]; ok {
				fieldKeys = append(fieldKeys, key)
				continue
			}
			if _, ok := brandUpdateRelations[key]; !ok {
				return nil, ErrInvalidBrandInput
			}
		}
	}

	sort.Strings(fieldKeys)

	for _, key := range fieldKeys {
		columns = append(columns, key)
		args = append(args, values[key])
		placeholders = append(
			placeholders,
			fmt.Sprintf("$%d", len(args)),
		)
	}

	query := fmt.Sprintf(
		"INSERT INTO brands (%s) VALUES (%s) RETURNING id",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	var brandID int64

	if err := tx.QueryRow(ctx, query, args...).Scan(&brandID); err != nil {
		if isBrandInputConstraintError(err) {
			return nil, ErrInvalidBrandInput
		}
		return nil, fmt.Errorf("insert brand: %w", err)
	}

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

		insertSQL := fmt.Sprintf(
			"INSERT INTO %s (brand_id, %s) VALUES ($1, $2)",
			relation.Table,
			relation.IDColumn,
		)

		for _, id := range ids {
			if _, err := tx.Exec(ctx, insertSQL, brandID, id); err != nil {
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

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit brand create: %w", err)
	}

	return r.FindDetailByID(ctx, brandID, companyID)
}
