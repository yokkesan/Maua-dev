package brand

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBrandNotFound = errors.New("brand not found")
)

type Repository interface {
	FindByCompanyID(
		ctx context.Context,
		companyID int64,
	) (*Brand, error)

	FindDetailByID(
		ctx context.Context,
		brandID int64,
		companyID int64,
	) (map[string]any, error)

	ListByCompanyID(
		ctx context.Context,
		companyID int64,
	) ([]Brand, error)

	UpdateByID(
		ctx context.Context,
		brandID int64,
		companyID int64,
		values map[string]any,
	) (map[string]any, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(
	db *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) FindByCompanyID(
	ctx context.Context,
	companyID int64,
) (*Brand, error) {
	var brand Brand

	err := r.db.QueryRow(
		ctx,
		`
			SELECT
				id,
				company_id,
				name,
				phonetic,
				tel,
				email
			FROM brands
			WHERE company_id = $1
			LIMIT 1
		`,
		companyID,
	).Scan(
		&brand.ID,
		&brand.CompanyID,
		&brand.Name,
		&brand.Phonetic,
		&brand.Tel,
		&brand.Email,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBrandNotFound
		}

		return nil, fmt.Errorf(
			"find brand by company id: %w",
			err,
		)
	}

	return &brand, nil
}

// ListByCompanyID は、指定された会社に所属するブランドをすべて取得する。
func (r *PostgresRepository) ListByCompanyID(
	ctx context.Context,
	companyID int64,
) ([]Brand, error) {
	const query = `
		SELECT
			id,
			company_id,
			name,
			phonetic,
			tel,
			email
		FROM brands
		WHERE company_id = $1
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query, companyID)
	if err != nil {
		return nil, fmt.Errorf("list brands by company id: %w", err)
	}
	defer rows.Close()

	brands := make([]Brand, 0)

	for rows.Next() {
		var b Brand

		if err := rows.Scan(
			&b.ID,
			&b.CompanyID,
			&b.Name,
			&b.Phonetic,
			&b.Tel,
			&b.Email,
		); err != nil {
			return nil, fmt.Errorf("scan brand: %w", err)
		}

		brands = append(brands, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brands: %w", err)
	}

	return brands, nil
}
