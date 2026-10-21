package pricescreening

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// kind is always chosen by selectionKind, never from a client-provided SQL identifier.
func readSelection(ctx context.Context, tx pgx.Tx, kind string, id int64) (*SelectionResult, error) {
	result := &SelectionResult{Products: []ProductSelection{}}
	var savedAt *time.Time
	err := tx.QueryRow(ctx, "SELECT h.first_category_id,h.saved_at,e.price_zone_id FROM "+kind+"s e LEFT JOIN "+kind+"_price_screenings h ON h."+kind+"_id=e.id WHERE e.id=$1", id).Scan(&result.FirstID, &savedAt, &result.PriceZoneID)
	if err != nil {
		return nil, err
	}
	result.Saved = savedAt != nil
	rows, err := tx.Query(ctx, "SELECT second_category_id,third_category_id,price_zone_guideline_id FROM "+kind+"_price_screening_products WHERE "+kind+"_id=$1 ORDER BY slot_number", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var product ProductSelection
		if err := rows.Scan(&product.SecondID, &product.ThirdID, &product.GuidelineID); err != nil {
			return nil, err
		}
		result.Products = append(result.Products, product)
	}
	return result, rows.Err()
}

func validateSelection(ctx context.Context, tx pgx.Tx, kind string, id int64, input SelectionInput, complete bool) ([]Guideline, error) {
	if input.FirstID <= 0 || input.Products == nil || len(input.Products) > 3 {
		return nil, ErrInvalidInput
	}
	var firstID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM first_categories WHERE id=$1 FOR SHARE`, input.FirstID).Scan(&firstID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidInput
		}
		return nil, err
	}
	var genreEligible bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+kind+"_categories ec JOIN categories c ON c.id=ec.category_id JOIN second_categories s ON s.id=c.second_id AND s.first_category_id=c.first_id WHERE ec."+kind+"_id=$1 AND c.first_id=$2)", id, input.FirstID).Scan(&genreEligible); err != nil {
		return nil, err
	}
	if !genreEligible {
		return nil, ErrInvalidInput
	}
	keys, names, answers := map[string]bool{}, map[string]bool{}, map[int64]bool{}
	guidelines := make([]Guideline, 0, len(input.Products))
	for _, product := range input.Products {
		if (product.SecondID == nil) == (product.ThirdID == nil) {
			return nil, ErrInvalidInput
		}
		var secondID, genre int64
		var secondName, name, key string
		if product.SecondID != nil {
			if *product.SecondID <= 0 {
				return nil, ErrInvalidInput
			}
			err := tx.QueryRow(ctx, `SELECT id,first_category_id,name FROM second_categories WHERE id=$1 FOR SHARE`, *product.SecondID).Scan(&secondID, &genre, &secondName)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvalidInput
			}
			if err != nil {
				return nil, err
			}
			if genre == 1 && secondName != "その他" {
				return nil, ErrInvalidInput
			}
			name, key = secondName, fmt.Sprintf("second:%d", secondID)
		} else {
			if *product.ThirdID <= 0 {
				return nil, ErrInvalidInput
			}
			err := tx.QueryRow(ctx, `SELECT s.id,s.first_category_id,s.name,th.name FROM third_categories th JOIN second_categories s ON s.id=th.second_category_id WHERE th.id=$1 FOR SHARE OF th,s`, *product.ThirdID).Scan(&secondID, &genre, &secondName, &name)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrInvalidInput
			}
			if err != nil {
				return nil, err
			}
			if genre != 1 || secondName == "その他" {
				return nil, ErrInvalidInput
			}
			key = fmt.Sprintf("third:%d", *product.ThirdID)
		}
		if genre != input.FirstID || keys[key] || names[name] {
			return nil, ErrInvalidInput
		}
		keys[key], names[name] = true, true
		var eligible bool
		query := `SELECT EXISTS (SELECT 1 FROM ` + kind + `_categories ec JOIN categories c ON c.id=ec.category_id
			WHERE ec.` + kind + `_id=$1 AND c.first_id=$2 AND c.second_id=$3
			AND (($4::bigint IS NULL AND c.third_id IS NULL) OR c.third_id=$4))`
		if err := tx.QueryRow(ctx, query, id, genre, secondID, product.ThirdID).Scan(&eligible); err != nil {
			return nil, err
		}
		if !eligible {
			return nil, ErrInvalidInput
		}
		if product.GuidelineID == nil {
			if complete {
				return nil, ErrInvalidInput
			}
			continue
		}
		if *product.GuidelineID <= 0 || answers[*product.GuidelineID] {
			return nil, ErrInvalidInput
		}
		answers[*product.GuidelineID] = true
		var rank int16
		var second, third *int64
		err := tx.QueryRow(ctx, `SELECT price_rank,second_id,third_id FROM price_zone_guidelines WHERE id=$1 FOR SHARE`, *product.GuidelineID).Scan(&rank, &second, &third)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidInput
		}
		if err != nil {
			return nil, err
		}
		if !sameOptionalID(second, product.SecondID) || !sameOptionalID(third, product.ThirdID) {
			return nil, ErrInvalidInput
		}
		if rank < 1 || rank > 3 {
			return nil, ErrInvalidMaster
		}
		guidelines = append(guidelines, Guideline{ID: *product.GuidelineID, Rank: rank, FirstID: genre, ProductKey: name, Eligible: true})
	}
	return guidelines, nil
}

func sameOptionalID(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func saveSelection(ctx context.Context, tx pgx.Tx, kind string, id int64, input SelectionInput, zone *int64) error {
	if zone != nil {
		var name string
		if err := tx.QueryRow(ctx, `SELECT name FROM price_zones WHERE id=$1 FOR SHARE`, *zone).Scan(&name); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidMaster
			}
			return err
		}
		if map[int64]string{1: "Standard", 2: "Highclass", 3: "Luxury"}[*zone] != name {
			return ErrInvalidMaster
		}
	}
	_, err := tx.Exec(ctx, "INSERT INTO "+kind+"_price_screenings ("+kind+"_id,first_category_id,saved_at) VALUES ($1,$2,NOW()) ON CONFLICT ("+kind+"_id) DO UPDATE SET first_category_id=EXCLUDED.first_category_id,saved_at=NOW(),updated_at=NOW()", id, input.FirstID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM "+kind+"_price_screening_products WHERE "+kind+"_id=$1", id); err != nil {
		return err
	}
	if kind == "brand" {
		if _, err := tx.Exec(ctx, `DELETE FROM brand_price_zone_guidelines WHERE brand_id=$1`, id); err != nil {
			return err
		}
	}
	for slot, product := range input.Products {
		_, err := tx.Exec(ctx, "INSERT INTO "+kind+"_price_screening_products ("+kind+"_id,slot_number,second_category_id,third_category_id,price_zone_guideline_id) VALUES ($1,$2,$3,$4,$5)", id, slot+1, product.SecondID, product.ThirdID, product.GuidelineID)
		if err != nil {
			return err
		}
		if kind == "brand" && product.GuidelineID != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO brand_price_zone_guidelines(brand_id,price_zone_guideline_id) VALUES ($1,$2)`, id, *product.GuidelineID); err != nil {
				return err
			}
		}
	}
	_, err = tx.Exec(ctx, "UPDATE "+kind+"s SET price_zone_id=COALESCE($1::bigint,price_zone_id),updated_at=NOW() WHERE id=$2", zone, id)
	return err
}
