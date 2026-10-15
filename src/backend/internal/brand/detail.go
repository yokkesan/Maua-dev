package brand

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// FindDetailByID は、指定された会社が所有するブランドの
// 基本情報と関連情報を取得する。
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

	rows, err := r.db.Query(
		ctx,
		query,
		brandID,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"find brand detail: %w",
			err,
		)
	}
	defer rows.Close()

	result, err := pgx.CollectOneRow(
		rows,
		pgx.RowToMap,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}

		return nil, fmt.Errorf(
			"scan brand detail: %w",
			err,
		)
	}

	// 関連情報の取得。
	// 更新APIと同じ定義を使用する。
	for key, relation := range brandUpdateRelations {
		ids, err := r.findBrandRelationIDs(
			ctx,
			brandID,
			relation,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"find brand relation %s: %w",
				key,
				err,
			)
		}

		result[key] = ids
	}

	return result, nil
}

// findBrandRelationIDs は関連テーブルからID一覧を取得する。
func (r *PostgresRepository) findBrandRelationIDs(
	ctx context.Context,
	brandID int64,
	relation brandRelation,
) ([]int64, error) {

	query := fmt.Sprintf(
		`
			SELECT %s
			FROM %s
			WHERE brand_id = $1
			ORDER BY %s
		`,
		relation.IDColumn,
		relation.Table,
		relation.IDColumn,
	)

	rows, err := r.db.Query(
		ctx,
		query,
		brandID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)

	for rows.Next() {
		var id int64

		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// GetDetailByID はブランド詳細取得処理を呼び出す。
func (s *Service) GetDetailByID(
	ctx context.Context,
	brandID int64,
	companyID int64,
) (map[string]any, error) {
	return s.repository.FindDetailByID(
		ctx,
		brandID,
		companyID,
	)
}
