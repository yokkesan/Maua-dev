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
