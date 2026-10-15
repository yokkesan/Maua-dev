package masters

type FirstCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SecondCategory struct {
	ID              int64  `json:"id"`
	FirstCategoryID int64  `json:"first_category_id"`
	Name            string `json:"name"`
}

type ThirdCategory struct {
	ID               int64  `json:"id"`
	SecondCategoryID int64  `json:"second_category_id"`
	Name             string `json:"name"`
}

type AreaRegion struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AreaPrefecture struct {
	ID           int64  `json:"id"`
	AreaRegionID int64  `json:"area_region_id"`
	Name         string `json:"name"`
}

type AreaCity struct {
	ID               int64  `json:"id"`
	AreaPrefectureID int64  `json:"area_prefecture_id"`
	Name             string `json:"name"`
}

type PriceZone struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type PriceZoneGuideline struct {
	ID         int64  `json:"id"`
	SecondID   int64  `json:"second_id"`
	ThirdID    *int64 `json:"third_id"`
	PriceRank  int16  `json:"price_rank"`
	PriceRange string `json:"price_range"`
}

type MasterItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type RegistrationMasters struct {
	FirstCategories   []FirstCategory      `json:"first_categories"`
	SecondCategories  []SecondCategory     `json:"second_categories"`
	ThirdCategories   []ThirdCategory      `json:"third_categories"`
	AreaRegions       []AreaRegion         `json:"area_regions"`
	AreaPrefectures   []AreaPrefecture     `json:"area_prefectures"`
	AreaCities        []AreaCity           `json:"area_cities"`
	PriceZones        []PriceZone          `json:"price_zones"`
	PriceGuidelines   []PriceZoneGuideline `json:"price_zone_guidelines"`
	SalesRecruitments []MasterItem         `json:"sales_recruitments"`
	SalesStyles       []MasterItem         `json:"sales_styles"`
	Orders            []MasterItem         `json:"orders"`
	PayRules          []MasterItem         `json:"pay_rules"`
}
