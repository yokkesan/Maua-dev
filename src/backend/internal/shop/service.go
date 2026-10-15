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
