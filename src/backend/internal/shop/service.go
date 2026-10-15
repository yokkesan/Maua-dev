package shop

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
) (*Shop, error) {
	result, err := s.repository.FindByCompanyID(
		ctx,
		companyID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get shop by company id: %w",
			err,
		)
	}

	return result, nil
}

// ListByCompanyID は、指定された会社のショップ一覧を取得する。
func (s *Service) ListByCompanyID(
	ctx context.Context,
	companyID int64,
) ([]Shop, error) {
	return s.repository.ListByCompanyID(ctx, companyID)
}
