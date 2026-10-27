package review

import (
	"context"
	"encoding/json"
	"errors"

	"backend/internal/notification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("review conflict")
var ErrInput = errors.New("invalid review input")

type Repository struct{ DB *pgxpool.Pool }

func valid(kind string) bool { return kind == "brands" || kind == "shops" }
func (r *Repository) List(ctx context.Context, kind string, before int64) ([]json.RawMessage, error) {
	if !valid(kind) || before < 0 {
		return nil, ErrInput
	}
	rows, err := r.DB.Query(ctx, `SELECT jsonb_build_object('id',e.id,'company_id',e.company_id,'name',e.name,'submitted_at',e.submitted_at,'approval',e.approval,'status',e.status) FROM `+kind+` e JOIN companies c ON c.id=e.company_id AND c.company_type=$2 WHERE e.submitted_at IS NOT NULL AND e.approval=0 AND ($1::bigint=0 OR e.id<$1) ORDER BY e.id DESC LIMIT 50`, before, companyType(kind))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(b))
	}
	return results, rows.Err()
}
func companyType(kind string) int16 {
	if kind == "brands" {
		return 1
	}
	return 0
}
func (r *Repository) Detail(ctx context.Context, kind string, id int64) (json.RawMessage, error) {
	if !valid(kind) || id <= 0 {
		return nil, ErrInput
	}
	singular := kind[:len(kind)-1]
	var b []byte
	err := r.DB.QueryRow(ctx, `SELECT jsonb_build_object('entity',to_jsonb(e),'company',to_jsonb(c),
 'categories',COALESCE((SELECT jsonb_agg(to_jsonb(cat)) FROM `+singular+`_categories ec JOIN categories cat ON cat.id=ec.category_id WHERE ec.`+singular+`_id=e.id),'[]'::jsonb),
 'images',COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY slot_number) FROM `+singular+`_images i WHERE i.`+singular+`_id=e.id),'[]'::jsonb),
 'price_screening',(SELECT jsonb_build_object('first_category_id',h.first_category_id,'saved',h.saved_at IS NOT NULL,'products',COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY slot_number) FROM `+singular+`_price_screening_products p WHERE p.`+singular+`_id=e.id),'[]'::jsonb)) FROM `+singular+`_price_screenings h WHERE h.`+singular+`_id=e.id))
 FROM `+kind+` e JOIN companies c ON c.id=e.company_id AND c.company_type=$2 WHERE e.id=$1 AND e.submitted_at IS NOT NULL`, id, companyType(kind)).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return json.RawMessage(b), err
}
func (r *Repository) Decide(ctx context.Context, kind string, id, companyID, adminID int64, approval int16) error {
	if !valid(kind) || id <= 0 || companyID <= 0 || adminID <= 0 || (approval != 1 && approval != 2) {
		return ErrInput
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var current int16
	var submitted bool
	err = tx.QueryRow(ctx, `SELECT e.approval,e.submitted_at IS NOT NULL FROM `+kind+` e JOIN companies c ON c.id=e.company_id AND c.company_type=$3 WHERE e.id=$1 AND e.company_id=$2 FOR UPDATE OF e`, id, companyID, companyType(kind)).Scan(&current, &submitted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !submitted || current != 0 {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `UPDATE `+kind+` SET approval=$1,updated_at=NOW() WHERE id=$2 AND company_id=$3`, approval, id, companyID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO review_decisions(entity,target_id,company_id,admin_user_id,approval) VALUES ($1,$2,$3,$4,$5)`, kind, id, companyID, adminID, approval); err != nil {
		return err
	}
	if err = notification.Decision(ctx, tx, kind, id, companyID, approval); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
