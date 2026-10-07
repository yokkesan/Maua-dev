package registration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrVerificationTokenNotFound = errors.New("verification token not found")

type Repository interface {
	CreateVerificationToken(
		ctx context.Context,
		params CreateTokenParams,
	) error

	FindVerificationTokenByHash(
		ctx context.Context,
		tokenHash []byte,
	) (*VerificationToken, error)

	CompleteRegistration(
		ctx context.Context,
		params CompleteRegistrationParams,
	) (*CompleteRegistrationResult, error)
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
		return fmt.Errorf(
			"begin registration transaction: %w",
			err,
		)
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
		return fmt.Errorf(
			"revoke existing signup tokens: %w",
			err,
		)
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
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9,
				$10,
				$11,
				$12
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
		return fmt.Errorf(
			"insert signup token: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf(
			"commit registration transaction: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) FindVerificationTokenByHash(
	ctx context.Context,
	tokenHash []byte,
) (*VerificationToken, error) {
	var token VerificationToken

	err := r.db.QueryRow(
		ctx,
		`
			SELECT
				id,
				invitation_email,
				company_name,
				company_phonetic,
				company_post_code,
				company_address,
				company_tel,
				company_type,
				representative_name,
				representative_phonetic,
				registration_plan,
				expires_at,
				consumed_at,
				revoked_at
			FROM signup_tokens
			WHERE token_hash = $1
			LIMIT 1
		`,
		tokenHash,
	).Scan(
		&token.ID,
		&token.InvitationEmail,
		&token.CompanyName,
		&token.CompanyPhonetic,
		&token.CompanyPostCode,
		&token.CompanyAddress,
		&token.CompanyTel,
		&token.CompanyType,
		&token.RepresentativeName,
		&token.RepresentativePhonetic,
		&token.RegistrationPlan,
		&token.ExpiresAt,
		&token.ConsumedAt,
		&token.RevokedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVerificationTokenNotFound
		}

		return nil, fmt.Errorf(
			"find verification token by hash: %w",
			err,
		)
	}

	return &token, nil
}

func (r *PostgresRepository) CompleteRegistration(
	ctx context.Context,
	params CompleteRegistrationParams,
) (*CompleteRegistrationResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"begin complete registration transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var companyID int64

	err = tx.QueryRow(
		ctx,
		`
			INSERT INTO companies (
				name,
				phonetic,
				post_code,
				address,
				tel,
				company_type,
				representative_name,
				representative_phonetic
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8
			)
			RETURNING id
		`,
		params.CompanyName,
		params.CompanyPhonetic,
		params.CompanyPostCode,
		params.CompanyAddress,
		params.CompanyTel,
		params.CompanyType,
		params.RepresentativeName,
		params.RepresentativePhonetic,
	).Scan(&companyID)

	if err != nil {
		return nil, fmt.Errorf(
			"insert company: %w",
			err,
		)
	}

	userType := UserTypeShop

	if params.CompanyType == CompanyTypeBrand {
		userType = UserTypeBrand
	}

	var userID int64

	err = tx.QueryRow(
		ctx,
		`
			INSERT INTO users (
				company_id,
				email,
				password_hash,
				user_type
			)
			VALUES (
				$1,
				$2,
				$3,
				$4
			)
			RETURNING id
		`,
		companyID,
		params.Email,
		params.PasswordHash,
		userType,
	).Scan(&userID)

	if err != nil {
		return nil, fmt.Errorf(
			"insert user: %w",
			err,
		)
	}

	result := &CompleteRegistrationResult{
		CompanyID: companyID,
		UserID:    userID,
	}

	switch params.CompanyType {
	case CompanyTypeBrand:
		var brandID int64

		err = tx.QueryRow(
			ctx,
			`
				INSERT INTO brands (
					company_id,
					name,
					phonetic,
					tel,
					email
				)
				VALUES (
					$1,
					$2,
					$3,
					$4,
					$5
				)
				RETURNING id
			`,
			companyID,
			params.CompanyName,
			params.CompanyPhonetic,
			params.CompanyTel,
			params.Email,
		).Scan(&brandID)

		if err != nil {
			return nil, fmt.Errorf(
				"insert brand: %w",
				err,
			)
		}

		result.BrandID = &brandID

	case CompanyTypeShop:
		var shopID int64

		err = tx.QueryRow(
			ctx,
			`
				INSERT INTO shops (
					company_id,
					name,
					phonetic,
					tel,
					email,
					registration_plan
				)
				VALUES (
					$1,
					$2,
					$3,
					$4,
					$5,
					$6
				)
				RETURNING id
			`,
			companyID,
			params.CompanyName,
			params.CompanyPhonetic,
			params.CompanyTel,
			params.Email,
			params.RegistrationPlan,
		).Scan(&shopID)

		if err != nil {
			return nil, fmt.Errorf(
				"insert shop: %w",
				err,
			)
		}

		result.ShopID = &shopID

	default:
		return nil, fmt.Errorf(
			"unsupported company type: %d",
			params.CompanyType,
		)
	}

	commandTag, err := tx.Exec(
		ctx,
		`
			UPDATE signup_tokens
			SET consumed_at = $1
			WHERE id = $2
			  AND consumed_at IS NULL
			  AND revoked_at IS NULL
		`,
		time.Now().UTC(),
		params.TokenID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consume signup token: %w",
			err,
		)
	}

	if commandTag.RowsAffected() != 1 {
		return nil, fmt.Errorf(
			"signup token could not be consumed",
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf(
			"commit complete registration transaction: %w",
			err,
		)
	}

	return result, nil
}