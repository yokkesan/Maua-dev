package masters

import "context"

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) GetAll(
	ctx context.Context,
) (*RegistrationMasters, error) {

	// DBからカテゴリー・価格帯を取得
	result, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Goで管理する固定マスターを追加
	result.AreaRegions = append([]AreaRegion{}, areaRegions...)
	result.AreaPrefectures = append([]AreaPrefecture{}, areaPrefectures...)
	result.AreaCities = append([]AreaCity{}, areaCities...)

	result.SalesRecruitments = append([]MasterItem{}, salesRecruitments...)
	result.SalesStyles = append([]MasterItem{}, salesStyles...)
	result.Orders = append([]MasterItem{}, orders...)
	result.PayRules = append([]MasterItem{}, payRules...)

	return result, nil
}
