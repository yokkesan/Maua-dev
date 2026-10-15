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

	// PostgreSQLからカテゴリー・価格帯を取得
	result, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// CSVから都道府県・市区町村を取得
	prefectures, cities, err := GetMunicipalities()
	if err != nil {
		return nil, err
	}

	// Goコードで管理する地方
	result.AreaRegions = append(
		[]AreaRegion{},
		areaRegions...,
	)

	// CSVで管理する都道府県・市区町村
	result.AreaPrefectures = prefectures
	result.AreaCities = cities

	// Goコードで管理する固定マスター
	result.SalesRecruitments = append(
		[]MasterItem{},
		salesRecruitments...,
	)

	result.SalesStyles = append(
		[]MasterItem{},
		salesStyles...,
	)

	result.Orders = append(
		[]MasterItem{},
		orders...,
	)

	result.PayRules = append(
		[]MasterItem{},
		payRules...,
	)

	return result, nil
}
