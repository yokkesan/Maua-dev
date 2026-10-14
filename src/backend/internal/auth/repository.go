package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("session not found")
)

type Repository interface {
	FindUserByEmail(
		ctx context.Context,
		email string,
	) (*User, error)

	CreateSession(
		ctx context.Context,
		session Session,
	) error

	FindUserBySessionTokenHash(
		ctx context.Context,
		tokenHash []byte,
	) (*User, error)

	DeleteSessionByTokenHash(
		ctx context.Context,
		tokenHash []byte,
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

func (r *PostgresRepository) FindUserByEmail(
	ctx context.Context,
	email string,
) (*User, error) {
	var user User

	err := r.db.QueryRow(
		ctx,
		`
			SELECT
				id,
				company_id,
				email,
				password_hash,
				user_type,
				role
			FROM users
			WHERE email = $1
			LIMIT 1
		`,
		email,
	).Scan(
		&user.ID,
		&user.CompanyID,
		&user.Email,
		&user.PasswordHash,
		&user.UserType,
		&user.Role,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}

		return nil, fmt.Errorf(
			"find user by email: %w",
			err,
		)
	}

	return &user, nil
}

func (r *PostgresRepository) CreateSession(
	ctx context.Context,
	session Session,
) error {
	_, err := r.db.Exec(
		ctx,
		`
			INSERT INTO user_sessions (
				user_id,
				token_hash,
				expires_at
			)
			VALUES (
				$1,
				$2,
				$3
			)
		`,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
	)

	if err != nil {
		return fmt.Errorf(
			"create user session: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) FindUserBySessionTokenHash(
	ctx context.Context,
	tokenHash []byte,
) (*User, error) {
	var user User

	err := r.db.QueryRow(
		ctx,
		`
			SELECT
				u.id,
				u.company_id,
				u.email,
				u.password_hash,
				u.user_type,
				u.role
			FROM user_sessions AS s
			INNER JOIN users AS u
				ON u.id = s.user_id
			WHERE s.token_hash = $1
				AND s.expires_at > NOW()
			LIMIT 1
		`,
		tokenHash,
	).Scan(
		&user.ID,
		&user.CompanyID,
		&user.Email,
		&user.PasswordHash,
		&user.UserType,
		&user.Role,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}

		return nil, fmt.Errorf(
			"find user by session token hash: %w",
			err,
		)
	}

	return &user, nil
}

func (r *PostgresRepository) DeleteSessionByTokenHash(
	ctx context.Context,
	tokenHash []byte,
) error {
	_, err := r.db.Exec(
		ctx,
		`
			DELETE FROM user_sessions
			WHERE token_hash = $1
		`,
		tokenHash,
	)

	if err != nil {
		return fmt.Errorf(
			"delete session by token hash: %w",
			err,
		)
	}

	return nil
}
