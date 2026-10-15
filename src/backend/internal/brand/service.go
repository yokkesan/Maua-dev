package brand

import (
	"context"
	"fmt"
)

type Service struct {
	repository Repository
}

func NewService(
	repository Repository,
) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) GetByCompanyID(
	ctx context.Context,
	companyID int64,
) (*Brand, error) {
	brand, err := s.repository.FindByCompanyID(
		ctx,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get brand by company id: %w",
			err,
		)
	}

	return brand, nil
}

// ListByCompanyID は、指定された会社のブランド一覧を取得する。
func (s *Service) ListByCompanyID(
	ctx context.Context,
	companyID int64,
) ([]Brand, error) {
	return s.repository.ListByCompanyID(ctx, companyID)
}
