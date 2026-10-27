package review

import (
	"context"
	"encoding/json"
)

type Service struct{ Repository *Repository }

func (s *Service) List(ctx context.Context, kind string, before int64) ([]json.RawMessage, error) {
	return s.Repository.List(ctx, kind, before)
}
func (s *Service) Detail(ctx context.Context, kind string, id int64) (json.RawMessage, error) {
	return s.Repository.Detail(ctx, kind, id)
}
func (s *Service) Decide(ctx context.Context, kind string, id, companyID, adminID int64, approval int16) error {
	if !valid(kind) || id <= 0 || companyID <= 0 || adminID <= 0 || (approval != 1 && approval != 2) {
		return ErrInput
	}
	return s.Repository.Decide(ctx, kind, id, companyID, adminID, approval)
}
