package registrationsubmission

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/internal/auth"
	"backend/internal/notification"
	"backend/internal/pricescreening"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db            *pgxpool.Pool
	notifications *notification.Config
}

func (r *PostgresRepository) WithNotifications(config notification.Config) *PostgresRepository {
	r.notifications = &config
	return r
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// SQL identifiers are selected exclusively from this server-side allowlist.
func tableFor(entity Entity) (string, error) {
	switch entity {
	case Brand:
		return "brands", nil
	case Shop:
		return "shops", nil
	default:
		return "", ErrInvalidInput
	}
}

func scanState(row pgx.Row) (*State, error) {
	var state State
	if err := row.Scan(&state.ID, &state.SubmittedAt, &state.Approval, &state.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read submission state: %w", err)
	}
	return &state, nil
}

func (r *PostgresRepository) Get(ctx context.Context, entity Entity, id, companyID int64) (*State, error) {
	table, err := tableFor(entity)
	if err != nil {
		return nil, err
	}
	return scanState(r.db.QueryRow(ctx,
		"SELECT id, submitted_at, approval, status FROM "+table+" WHERE id = $1 AND company_id = $2",
		id, companyID))
}

func (r *PostgresRepository) Submit(ctx context.Context, entity Entity, id, companyID int64) (*State, error) {
	table, err := tableFor(entity)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin submission: %w", err)
	}
	// Use a separate bounded context so cancellation cannot prevent cleanup.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	state, err := scanState(tx.QueryRow(ctx,
		"SELECT id, submitted_at, approval, status FROM "+table+" WHERE id = $1 AND company_id = $2 FOR UPDATE",
		id, companyID))
	if err != nil {
		return nil, err
	}
	if state.SubmittedAt != nil || state.Approval != 0 || state.Status != 0 {
		return nil, ErrConflict
	}
	info, err := loadRequiredInformation(ctx, tx, table, entity, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := validateRequiredInformation(entity, *info); err != nil {
		return nil, err
	}
	if err := pricescreening.ValidateSavedSelection(ctx, tx, table, id); err != nil {
		if errors.Is(err, pricescreening.ErrInvalidInput) || errors.Is(err, pricescreening.ErrInvalidMaster) {
			return nil, &RequiredFieldsError{Fields: []string{"price_screening"}}
		}
		return nil, fmt.Errorf("validate submission price screening: %w", err)
	}
	state, err = scanState(tx.QueryRow(ctx,
		"UPDATE "+table+" SET submitted_at = NOW(), updated_at = NOW() WHERE id = $1 AND company_id = $2 RETURNING id, submitted_at, approval, status",
		id, companyID))
	if err != nil {
		return nil, err
	}
	if r.notifications != nil {
		actor, ok := auth.UserFromContext(ctx)
		if !ok || actor == nil {
			return nil, fmt.Errorf("submission actor required")
		}
		if err := notification.Submission(ctx, tx, *r.notifications, table, id, companyID, actor.ID); err != nil {
			return nil, fmt.Errorf("record submission notifications: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit submission: %w", err)
	}
	return state, nil
}

// Called only after the target row is locked and its submission state checked.
// All identifiers originate in tableFor; all record values use placeholders.
func loadRequiredInformation(ctx context.Context, tx pgx.Tx, table string, entity Entity, id, companyID int64) (*requiredInformation, error) {
	var info requiredInformation
	// Keep the owning company's values stable until submission commits.
	err := tx.QueryRow(ctx, `SELECT name, phonetic, representative_name,
		representative_phonetic, post_code, address, tel FROM companies WHERE id=$1 FOR SHARE`, companyID).Scan(
		&info.CompanyName, &info.CompanyPhonetic, &info.RepresentativeName,
		&info.RepresentativePhonetic, &info.PostCode, &info.Address, &info.Tel,
	)
	if err != nil {
		return nil, fmt.Errorf("load submission company: %w", err)
	}
	entityID := "shop_id"
	brandColumns := "NULL::text, NULL::text, NULL::text"
	if entity == Brand {
		entityID = "brand_id"
		brandColumns = "price_min, price_max, base"
	}
	query := `SELECT name, phonetic, concept, target_min, target_max, web_url, net_shop,
		contact_person_name, contact_person_phonetic, contact_person_tel, ` + brandColumns + `,
		EXISTS (SELECT 1 FROM ` + table[:len(table)-1] + `_images WHERE ` + entityID + `=$1 AND slot_number=1 AND storage_key <> '')
		AND EXISTS (SELECT 1 FROM ` + table[:len(table)-1] + `_images WHERE ` + entityID + `=$1 AND slot_number=2 AND storage_key <> ''),
		EXISTS (SELECT 1 FROM ` + table[:len(table)-1] + `_categories ec
			JOIN categories c ON c.id=ec.category_id
			JOIN second_categories s ON s.id=c.second_id AND s.first_category_id=c.first_id
			LEFT JOIN third_categories th ON th.id=c.third_id AND th.second_category_id=s.id
			WHERE ec.` + entityID + `=$1 AND (c.third_id IS NULL OR th.id IS NOT NULL))
		FROM ` + table + ` WHERE id=$1 AND company_id=$2`
	err = tx.QueryRow(ctx, query, id, companyID).Scan(
		&info.Name, &info.Phonetic, &info.Concept, &info.TargetMin, &info.TargetMax, &info.WebURL,
		&info.NetShop, &info.ContactName, &info.ContactPhonetic, &info.ContactTel,
		&info.PriceMin, &info.PriceMax, &info.Base, &info.MainImages, &info.Category,
	)
	if err != nil {
		return nil, fmt.Errorf("load submission details: %w", err)
	}
	return &info, nil
}
