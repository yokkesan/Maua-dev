package pricescreening

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

type brandTransaction struct {
	tx      pgx.Tx
	brandID int64
}

func (r *PostgresRepository) WithBrand(ctx context.Context, brandID, companyID int64, fn func(Transaction) error) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin price screening: %w", err)
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `SELECT id FROM brands WHERE id=$1 AND company_id=$2 FOR UPDATE`, brandID, companyID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock price screening brand: %w", err)
	}
	if err := fn(&brandTransaction{tx: tx, brandID: brandID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit price screening: %w", err)
	}
	return nil
}

// 1つのSELECTで回答と価格帯を同じスナップショットから取得する。
const resultQuery = `SELECT
    ARRAY(SELECT price_zone_guideline_id FROM brand_price_zone_guidelines
          WHERE brand_id=b.id ORDER BY price_zone_guideline_id), b.price_zone_id
    FROM brands b WHERE b.id=$1`

func (r *PostgresRepository) Get(ctx context.Context, brandID, companyID int64) (*Result, error) {
	var result Result
	err := r.db.QueryRow(ctx, resultQuery+` AND b.company_id=$2`, brandID, companyID).Scan(&result.GuidelineIDs, &result.PriceZoneID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get price screening: %w", err)
	}
	return &result, nil
}

func (t *brandTransaction) Current(ctx context.Context) (*Result, error) {
	var result Result
	if err := t.tx.QueryRow(ctx, resultQuery, t.brandID).Scan(&result.GuidelineIDs, &result.PriceZoneID); err != nil {
		return nil, fmt.Errorf("get current price screening: %w", err)
	}
	return &result, nil
}

func (t *brandTransaction) Guidelines(ctx context.Context, ids []int64) ([]Guideline, error) {
	rows, err := t.tx.Query(ctx, `SELECT g.id, g.price_rank, g.second_id, g.third_id,
        s.id, s.first_category_id, s.name, th.name,
        EXISTS (SELECT 1 FROM brand_categories bc JOIN categories c ON c.id=bc.category_id
            WHERE bc.brand_id=$2 AND c.first_id=s.first_category_id AND c.second_id=s.id
              AND ((g.third_id IS NOT NULL AND c.third_id=g.third_id)
                   OR (g.second_id IS NOT NULL AND c.third_id IS NULL)))
        FROM price_zone_guidelines g
        LEFT JOIN third_categories th ON th.id=g.third_id
        LEFT JOIN second_categories s ON s.id=COALESCE(g.second_id, th.second_category_id)
        WHERE g.id=ANY($1::bigint[]) FOR SHARE OF g`, ids, t.brandID)
	if err != nil {
		return nil, fmt.Errorf("load price guidelines: %w", err)
	}
	defer rows.Close()
	var guidelines []Guideline
	for rows.Next() {
		var g Guideline
		var secondID, thirdID, parentID, firstID *int64
		var secondName, thirdName *string
		if err := rows.Scan(&g.ID, &g.Rank, &secondID, &thirdID, &parentID, &firstID, &secondName, &thirdName, &g.Eligible); err != nil {
			return nil, fmt.Errorf("scan price guideline: %w", err)
		}
		if (secondID == nil) == (thirdID == nil) || parentID == nil || firstID == nil || secondName == nil {
			return nil, ErrInvalidMaster
		}
		g.FirstID = *firstID
		// Laravel: アパレル(1)は第三階層、第二階層名「その他」のみ第二階層。
		// 他ジャンルは第二階層。商品名の重複は選択肢側で除去されている。
		if thirdID != nil {
			if g.FirstID != 1 || *secondName == "その他" || thirdName == nil {
				return nil, ErrInvalidMaster
			}
			g.ProductKey = *thirdName
		} else {
			if g.FirstID == 1 && *secondName != "その他" {
				return nil, ErrInvalidMaster
			}
			g.ProductKey = *secondName
		}
		guidelines = append(guidelines, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read price guidelines: %w", err)
	}
	return guidelines, nil
}

func (t *brandTransaction) Save(ctx context.Context, ids []int64, zone int64) error {
	var name string
	err := t.tx.QueryRow(ctx, `SELECT name FROM price_zones WHERE id=$1 FOR SHARE`, zone).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidMaster
	}
	if err != nil {
		return fmt.Errorf("load price zone: %w", err)
	}
	names := map[int64]string{1: "Standard", 2: "Highclass", 3: "Luxury"}
	if names[zone] != name {
		return ErrInvalidMaster
	}
	if _, err := t.tx.Exec(ctx, `DELETE FROM brand_price_zone_guidelines WHERE brand_id=$1`, t.brandID); err != nil {
		return fmt.Errorf("delete price screening answers: %w", err)
	}
	if _, err := t.tx.Exec(ctx, `INSERT INTO brand_price_zone_guidelines (brand_id, price_zone_guideline_id)
        SELECT $1, unnest($2::bigint[])`, t.brandID, ids); err != nil {
		return fmt.Errorf("insert price screening answers: %w", err)
	}
	if _, err := t.tx.Exec(ctx, `UPDATE brands SET price_zone_id=$1, updated_at=NOW() WHERE id=$2`, zone, t.brandID); err != nil {
		return fmt.Errorf("update price screening zone: %w", err)
	}
	// A legacy answer update has no independent product selection. Invalidate
	// readiness atomically, preserving the previous selection for inspection.
	if _, err := t.tx.Exec(ctx, `UPDATE brand_price_screenings SET saved_at=NULL,updated_at=NOW() WHERE brand_id=$1`, t.brandID); err != nil {
		return fmt.Errorf("invalidate price screening selection: %w", err)
	}
	return nil
}
