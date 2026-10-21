package pricescreening

import (
	"context"
	"errors"
	"sort"
)

var (
	ErrNotFound      = errors.New("brand not found")
	ErrInvalidInput  = errors.New("invalid price screening input")
	ErrInvalidMaster = errors.New("inconsistent price screening master")
)

type Result struct {
	GuidelineIDs []int64 `json:"price_zone_guideline_ids"`
	PriceZoneID  *int64  `json:"price_zone_id"`
}

// Guidelineはクライアント値ではなく、トランザクション内で読み込んだマスター。
type Guideline struct {
	ID         int64
	Rank       int16
	FirstID    int64
	ProductKey string
	Eligible   bool
}

type Transaction interface {
	Guidelines(context.Context, []int64) ([]Guideline, error)
	Current(context.Context) (*Result, error)
	Save(context.Context, []int64, int64) error
}

type Repository interface {
	WithBrand(context.Context, int64, int64, func(Transaction) error) error
	Get(context.Context, int64, int64) (*Result, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Get(ctx context.Context, brandID, companyID int64) (*Result, error) {
	if brandID <= 0 || companyID <= 0 {
		return nil, ErrInvalidInput
	}
	return s.repository.Get(ctx, brandID, companyID)
}

func (s *Service) Put(ctx context.Context, brandID, companyID int64, ids []int64) (*Result, error) {
	if brandID <= 0 || companyID <= 0 {
		return nil, ErrInvalidInput
	}
	var result *Result
	err := s.repository.WithBrand(ctx, brandID, companyID, func(tx Transaction) error {
		// 所有確認・ブランドロック後に検証する。空配列は既存結果を維持する。
		if ids == nil || len(ids) > 3 {
			return ErrInvalidInput
		}
		seen := make(map[int64]bool, len(ids))
		for _, id := range ids {
			if id <= 0 || seen[id] {
				return ErrInvalidInput
			}
			seen[id] = true
		}
		if len(ids) == 0 {
			var err error
			result, err = tx.Current(ctx)
			return err
		}
		guidelines, err := tx.Guidelines(ctx, ids)
		if err != nil {
			return err
		}
		if len(guidelines) != len(ids) {
			return ErrInvalidInput
		}
		zone, err := calculate(guidelines)
		if err != nil {
			return err
		}
		if err := tx.Save(ctx, ids, zone); err != nil {
			return err
		}
		savedIDs := append([]int64{}, ids...)
		sort.Slice(savedIDs, func(i, j int) bool { return savedIDs[i] < savedIDs[j] })
		result = &Result{GuidelineIDs: savedIDs, PriceZoneID: &zone}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func calculate(guidelines []Guideline) (int64, error) {
	if len(guidelines) == 0 || len(guidelines) > 3 {
		return 0, ErrInvalidInput
	}
	var sum int64
	products := make(map[string]bool, len(guidelines))
	for _, g := range guidelines {
		if g.Rank < 1 || g.Rank > 3 || g.FirstID <= 0 || g.ProductKey == "" {
			return 0, ErrInvalidMaster
		}
		if !g.Eligible || g.FirstID != guidelines[0].FirstID || products[g.ProductKey] {
			return 0, ErrInvalidInput
		}
		products[g.ProductKey] = true
		sum += int64(g.Rank)
	}
	// 正の整数だけなので、浮動小数を使わずPHP roundの四捨五入を再現する。
	n := int64(len(guidelines))
	return (2*sum + n) / (2 * n), nil
}
