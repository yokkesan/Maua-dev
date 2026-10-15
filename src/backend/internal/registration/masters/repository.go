package masters

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAll(
	ctx context.Context,
) (*RegistrationMasters, error) {
	result := &RegistrationMasters{}

	var err error

	// 大カテゴリー
	result.FirstCategories, err = queryItems(
		ctx,
		r.db,
		"SELECT id, name FROM first_categories ORDER BY id",
		func(rows pgx.Rows) (FirstCategory, error) {
			var item FirstCategory
			err := rows.Scan(&item.ID, &item.Name)
			return item, err
		},
	)
	if err != nil {
		return nil, fmt.Errorf("first categories: %w", err)
	}

	// 中カテゴリー
	result.SecondCategories, err = queryItems(
		ctx,
		r.db,
		`SELECT id, first_category_id, name
		 FROM second_categories ORDER BY id`,
		func(rows pgx.Rows) (SecondCategory, error) {
			var item SecondCategory
			err := rows.Scan(
				&item.ID,
				&item.FirstCategoryID,
				&item.Name,
			)
			return item, err
		},
	)
	if err != nil {
		return nil, fmt.Errorf("second categories: %w", err)
	}

	// 小カテゴリー
	result.ThirdCategories, err = queryItems(
		ctx,
		r.db,
		`SELECT id, second_category_id, name
		 FROM third_categories ORDER BY id`,
		func(rows pgx.Rows) (ThirdCategory, error) {
			var item ThirdCategory
			err := rows.Scan(
				&item.ID,
				&item.SecondCategoryID,
				&item.Name,
			)
			return item, err
		},
	)
	if err != nil {
		return nil, fmt.Errorf("third categories: %w", err)
	}

	// 価格帯
	result.PriceZones, err = queryItems(
		ctx,
		r.db,
		"SELECT id, name FROM price_zones ORDER BY id",
		func(rows pgx.Rows) (PriceZone, error) {
			var item PriceZone
			err := rows.Scan(&item.ID, &item.Name)
			return item, err
		},
	)
	if err != nil {
		return nil, fmt.Errorf("price zones: %w", err)
	}

	// プライススクリーニング
	result.PriceGuidelines, err = queryItems(
		ctx,
		r.db,
		`SELECT id, second_id, third_id, price_rank, price_range
		 FROM price_zone_guidelines ORDER BY id`,
		func(rows pgx.Rows) (PriceZoneGuideline, error) {
			var item PriceZoneGuideline
			err := rows.Scan(
				&item.ID,
				&item.SecondID,
				&item.ThirdID,
				&item.PriceRank,
				&item.PriceRange,
			)
			return item, err
		},
	)
	if err != nil {
		return nil, fmt.Errorf("price guidelines: %w", err)
	}

	return result, nil
}

func queryItems[T any](
	ctx context.Context,
	db *pgxpool.Pool,
	query string,
	scan func(pgx.Rows) (T, error),
) ([]T, error) {
	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]T, 0)

	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
