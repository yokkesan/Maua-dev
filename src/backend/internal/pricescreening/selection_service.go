package pricescreening

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Product IDs refer directly to the existing second/third category masters.
// Exactly one category ID is supplied. A nil guideline represents an unanswered selection.
type ProductSelection struct {
	SecondID    *int64 `json:"second_category_id"`
	ThirdID     *int64 `json:"third_category_id"`
	GuidelineID *int64 `json:"price_zone_guideline_id"`
}

type SelectionInput struct {
	FirstID  int64              `json:"first_category_id"`
	Products []ProductSelection `json:"products"`
}

type SelectionResult struct {
	FirstID     *int64             `json:"first_category_id"`
	Products    []ProductSelection `json:"products"`
	Saved       bool               `json:"saved"`
	PriceZoneID *int64             `json:"price_zone_id"`
}

type SelectionService struct{ db *pgxpool.Pool }

func NewSelectionService(db *pgxpool.Pool) *SelectionService { return &SelectionService{db: db} }

func selectionKind(table string) (string, error) {
	switch table {
	case "brands":
		return "brand", nil
	case "shops":
		return "shop", nil
	default:
		return "", ErrInvalidInput
	}
}

func (s *SelectionService) Get(ctx context.Context, table string, id, companyID int64) (*SelectionResult, error) {
	return s.withTarget(ctx, table, id, companyID, nil)
}

func (s *SelectionService) Put(ctx context.Context, table string, id, companyID int64, input SelectionInput) (*SelectionResult, error) {
	return s.withTarget(ctx, table, id, companyID, &input)
}

func (s *SelectionService) withTarget(ctx context.Context, table string, id, companyID int64, input *SelectionInput) (*SelectionResult, error) {
	kind, err := selectionKind(table)
	if err != nil || id <= 0 || companyID <= 0 {
		return nil, ErrInvalidInput
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanupCtx)
	}()
	lock := " FOR SHARE"
	if input != nil {
		lock = " FOR UPDATE"
	}
	var existingID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM "+table+" WHERE id=$1 AND company_id=$2"+lock, id, companyID).Scan(&existingID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if input != nil {
		guidelines, err := validateSelection(ctx, tx, kind, id, *input, false)
		if err != nil {
			return nil, err
		}
		var zone *int64
		if len(guidelines) > 0 {
			value, err := calculate(guidelines)
			if err != nil {
				return nil, err
			}
			zone = &value
		}
		if err := saveSelection(ctx, tx, kind, id, *input, zone); err != nil {
			return nil, err
		}
	}
	result, err := readSelection(ctx, tx, kind, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateSavedSelection runs inside the caller's transaction after the entity
// row is locked. A header is necessary even for an explicitly saved empty selection.
func ValidateSavedSelection(ctx context.Context, tx pgx.Tx, table string, id int64) error {
	kind, err := selectionKind(table)
	if err != nil {
		return err
	}
	result, err := readSelection(ctx, tx, kind, id)
	if err != nil {
		return err
	}
	if !result.Saved || result.FirstID == nil {
		return ErrInvalidInput
	}
	_, err = validateSelection(ctx, tx, kind, id, SelectionInput{FirstID: *result.FirstID, Products: result.Products}, true)
	return err
}
