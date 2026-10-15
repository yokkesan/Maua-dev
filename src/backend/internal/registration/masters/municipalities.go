package masters

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

//go:embed data/localgovjp-utf8.csv
var municipalityCSV string

var (
	municipalitiesOnce sync.Once
	municipalitiesErr  error

	cachedPrefectures []AreaPrefecture
	cachedCities      []AreaCity
)

// 都道府県コードから地方IDを取得
func regionIDFromPrefectureID(id int64) (int64, error) {
	switch {
	case id == 1:
		return 1, nil
	case id >= 2 && id <= 7:
		return 2, nil
	case id >= 8 && id <= 14:
		return 3, nil
	case id >= 15 && id <= 23:
		return 4, nil
	case id >= 24 && id <= 30:
		return 5, nil
	case id >= 31 && id <= 35:
		return 6, nil
	case id >= 36 && id <= 39:
		return 7, nil
	case id >= 40 && id <= 47:
		return 8, nil
	default:
		return 0, fmt.Errorf("invalid prefecture id: %d", id)
	}
}

func loadMunicipalities() error {
	reader := csv.NewReader(strings.NewReader(municipalityCSV))

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("read municipalities CSV: %w", err)
	}

	if len(records) < 2 {
		return fmt.Errorf("municipalities CSV is empty")
	}

	header := records[0]

	if len(header) < 4 ||
		strings.TrimPrefix(header[0], "\uFEFF") != "pid" ||
		header[2] != "cid" ||
		header[3] != "city" {
		return fmt.Errorf("unexpected municipalities CSV format")
	}

	prefectures := make(map[int64]AreaPrefecture)
	cities := make([]AreaCity, 0, len(records)-1)
	cityIDs := make(map[int64]struct{})

	for line, record := range records[1:] {
		if len(record) < 4 {
			return fmt.Errorf("invalid CSV row: %d", line+2)
		}

		prefectureID, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid prefecture id at row %d: %w", line+2, err)
		}

		cityID, err := strconv.ParseInt(record[2], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid city id at row %d: %w", line+2, err)
		}

		if cityID <= 0 || record[3] == "" || record[1] == "" {
			return fmt.Errorf("invalid municipality at row %d", line+2)
		}

		if _, exists := cityIDs[cityID]; exists {
			return fmt.Errorf("duplicate city id: %d", cityID)
		}
		cityIDs[cityID] = struct{}{}

		regionID, err := regionIDFromPrefectureID(prefectureID)
		if err != nil {
			return err
		}

		if existing, exists := prefectures[prefectureID]; exists {
			if existing.Name != record[1] {
				return fmt.Errorf(
					"inconsistent prefecture name: %d",
					prefectureID,
				)
			}
		} else {
			prefectures[prefectureID] = AreaPrefecture{
				ID:           prefectureID,
				AreaRegionID: regionID,
				Name:         record[1],
			}
		}

		cities = append(cities, AreaCity{
			ID:               cityID,
			AreaPrefectureID: prefectureID,
			Name:             record[3],
		})
	}

	if len(prefectures) != 47 {
		return fmt.Errorf(
			"unexpected prefecture count: %d",
			len(prefectures),
		)
	}

	cachedPrefectures = make([]AreaPrefecture, 0, len(prefectures))

	for _, prefecture := range prefectures {
		cachedPrefectures = append(cachedPrefectures, prefecture)
	}

	sort.Slice(cachedPrefectures, func(i, j int) bool {
		return cachedPrefectures[i].ID < cachedPrefectures[j].ID
	})

	sort.Slice(cities, func(i, j int) bool {
		return cities[i].ID < cities[j].ID
	})

	cachedCities = cities

	return nil
}

// 都道府県・市区町村一覧を取得
func GetMunicipalities() ([]AreaPrefecture, []AreaCity, error) {
	municipalitiesOnce.Do(func() {
		municipalitiesErr = loadMunicipalities()
	})

	if municipalitiesErr != nil {
		return nil, nil, municipalitiesErr
	}

	prefectures := append(
		[]AreaPrefecture{},
		cachedPrefectures...,
	)

	cities := append(
		[]AreaCity{},
		cachedCities...,
	)

	return prefectures, cities, nil
}
