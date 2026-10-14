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
