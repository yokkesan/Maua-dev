package pricescreening

import (
	"errors"
	"fmt"
	"testing"
)

func TestCalculate(t *testing.T) {
	for _, tc := range []struct {
		ranks []int16
		want  int64
	}{
		{[]int16{1}, 1}, {[]int16{2}, 2}, {[]int16{3}, 3},
		{[]int16{1, 2}, 2}, {[]int16{2, 3}, 3}, {[]int16{1, 3}, 2},
		{[]int16{1, 1, 2}, 1}, {[]int16{1, 2, 2}, 2},
		{[]int16{2, 2, 3}, 2}, {[]int16{2, 3, 3}, 3}, {[]int16{1, 2, 3}, 2},
	} {
		t.Run(fmt.Sprint(tc.ranks), func(t *testing.T) {
			gs := make([]Guideline, len(tc.ranks))
			for i, rank := range tc.ranks {
				gs[i] = Guideline{ID: int64(i + 1), Rank: rank, FirstID: 2, ProductKey: fmt.Sprint(i), Eligible: true}
			}
			got, err := calculate(gs)
			if err != nil || got != tc.want {
				t.Fatalf("got %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestCalculateRejectsInvalidData(t *testing.T) {
	good := Guideline{ID: 1, Rank: 2, FirstID: 2, ProductKey: "a", Eligible: true}
	for _, tc := range []struct {
		name   string
		modify func(*Guideline)
		want   error
	}{
		{"zero rank", func(g *Guideline) { g.Rank = 0 }, ErrInvalidMaster},
		{"rank four", func(g *Guideline) { g.Rank = 4 }, ErrInvalidMaster},
		{"missing genre", func(g *Guideline) { g.FirstID = 0 }, ErrInvalidMaster},
		{"missing product", func(g *Guideline) { g.ProductKey = "" }, ErrInvalidMaster},
		{"other genre", func(g *Guideline) { g.FirstID = 3 }, ErrInvalidInput},
		{"other category", func(g *Guideline) { g.Eligible = false }, ErrInvalidInput},
		{"duplicate product", func(g *Guideline) { g.ProductKey = "a" }, ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := Guideline{ID: 2, Rank: 3, FirstID: 2, ProductKey: "b", Eligible: true}
			tc.modify(&other)
			_, err := calculate([]Guideline{good, other})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
