package shop

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrShopNotFound = errors.New("shop not found")

type Repository interface {
	FindByCompanyID(
		ctx context.Context,
		companyID int64,
	) (*Shop, error)
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
) (*Shop, error) {
	const query = `
		SELECT
			id,
			company_id,
			name,
			phonetic,
			tel,
			email,
			registration_plan
		FROM shops
		WHERE company_id = $1
		LIMIT 1
	`

	var shop Shop

	err := r.db.QueryRow(
		ctx,
		query,
		companyID,
	).Scan(
		&shop.ID,
		&shop.CompanyID,
		&shop.Name,
		&shop.Phonetic,
		&shop.Tel,
		&shop.Email,
		&shop.RegistrationPlan,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrShopNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"failed to find shop by company id: %w",
			err,
		)
	}

	return &shop, nil
}
