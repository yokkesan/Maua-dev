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

	FindDetailByID(
		ctx context.Context,
		shopID int64,
		companyID int64,
	) (map[string]any, error)

	ListByCompanyID(
		ctx context.Context,
		companyID int64,
	) ([]Shop, error)

	UpdateByID(
		ctx context.Context,
		shopID int64,
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

// ListByCompanyID は、指定された会社に所属するショップをすべて取得する。
func (r *PostgresRepository) ListByCompanyID(
	ctx context.Context,
	companyID int64,
) ([]Shop, error) {
	const query = `
		SELECT
			id,
			company_id,
			name,
			phonetic,
			tel,
			email
		FROM shops
		WHERE company_id = $1
		ORDER BY id
	`

	rows, err := r.db.Query(ctx, query, companyID)
	if err != nil {
		return nil, fmt.Errorf("list shops by company id: %w", err)
	}
	defer rows.Close()

	shops := make([]Shop, 0)

	for rows.Next() {
		var s Shop

		if err := rows.Scan(
			&s.ID,
			&s.CompanyID,
			&s.Name,
			&s.Phonetic,
			&s.Tel,
			&s.Email,
		); err != nil {
			return nil, fmt.Errorf("scan shop: %w", err)
		}

		shops = append(shops, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shops: %w", err)
	}

	return shops, nil
}
