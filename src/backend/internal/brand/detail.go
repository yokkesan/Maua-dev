package brand

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FindDetailByID は、指定された会社に所属するブランドの詳細を取得する。
func (r *PostgresRepository) FindDetailByID(
	ctx context.Context,
	brandID int64,
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
			concept,
			top_news,
			other_brand,
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
			price_min,
			price_max,
			base,
			rate_dm_form,
			rate_min,
			rate_max,
			box_price_min,
			box_price_max,
			one_gram,
			gram_price_min,
			gram_price_max,
			price_free_text,
			lot_free_text,
			price_zone_id,
			approval,
			status
		FROM brands
		WHERE id = $1
		  AND company_id = $2
	`

	rows, err := r.db.Query(ctx, query, brandID, companyID)
	if err != nil {
		return nil, fmt.Errorf("find brand detail: %w", err)
	}
	defer rows.Close()

	result, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}
		return nil, fmt.Errorf("scan brand detail: %w", err)
	}

	return result, nil
}

// GetDetailByID はブランド詳細取得処理を呼び出す。
func (s *Service) GetDetailByID(
	ctx context.Context,
	brandID int64,
	companyID int64,
) (map[string]any, error) {
	return s.repository.FindDetailByID(ctx, brandID, companyID)
}
