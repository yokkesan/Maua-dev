package registration

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateVerificationToken(
		ctx context.Context,
		params CreateTokenParams,
	) error
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

func (r *PostgresRepository) CreateVerificationToken(
	ctx context.Context,
	params CreateTokenParams,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registration transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(
		ctx,
		`
			UPDATE signup_tokens
			SET revoked_at = $1
			WHERE invitation_email = $2
			  AND consumed_at IS NULL
			  AND revoked_at IS NULL
		`,
		time.Now().UTC(),
		params.Email,
	)
	if err != nil {
		return fmt.Errorf("revoke existing signup tokens: %w", err)
	}

	_, err = tx.Exec(
		ctx,
		`
			INSERT INTO signup_tokens (
				invitation_email,
				token_hash,
				company_name,
				company_phonetic,
				company_post_code,
				company_address,
				company_tel,
				company_type,
				representative_name,
				representative_phonetic,
				registration_plan,
				expires_at
			)
			VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, $12
			)
		`,
		params.Email,
		params.TokenHash,
		params.CompanyName,
		params.CompanyPhonetic,
		params.CompanyPostCode,
		params.CompanyAddress,
		params.CompanyTel,
		params.CompanyType,
		params.RepresentativeName,
		params.RepresentativePhonetic,
		params.RegistrationPlan,
		params.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert signup token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration transaction: %w", err)
	}

	return nil
}