package shop

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FindDetailByID は、指定された会社に所属するショップの詳細を取得する。
func (r *PostgresRepository) FindDetailByID(
	ctx context.Context,
	shopID int64,
	companyID int64,
) (map[string]any, error) {

	const query = `
		SELECT
			id,
			company_id,
			name,
			phonetic,
			tel,
			email,
			registration_plan,
			concept,
			target_min,
			target_max,
			web_url,
			net_shop,
			contact_person_name,
			contact_person_phonetic,
			contact_person_tel,
			facebook,
			twitter,
			instagram,
			line,
			address,
			handling_brand,
			price_zone_id,
			approval,
			status
		FROM shops
		WHERE id = $1
		  AND company_id = $2
	`

	rows, err := r.db.Query(ctx, query, shopID, companyID)
	if err != nil {
		return nil, fmt.Errorf("find shop detail: %w", err)
	}
	defer rows.Close()

	result, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrShopNotFound
		}

		return nil, fmt.Errorf("scan shop detail: %w", err)
	}

	return result, nil
}

// GetDetailByID はショップ詳細取得処理を呼び出す。
func (s *Service) GetDetailByID(
	ctx context.Context,
	shopID int64,
	companyID int64,
) (map[string]any, error) {
	return s.repository.FindDetailByID(ctx, shopID, companyID)
}
