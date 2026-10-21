package registrationsubmission

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

type RequiredFieldsError struct {
	Fields []string
}

func (e *RequiredFieldsError) Error() string { return "required fields are missing" }

// Pointers preserve SQL NULL versus empty strings. target_min and net_shop
// follow the legacy Vue null-only checks; other required strings reject both.
type requiredInformation struct {
	CompanyName, CompanyPhonetic, RepresentativeName, RepresentativePhonetic *string
	PostCode, Address, Tel                                                   *string
	Name, Phonetic, Concept, TargetMin, TargetMax, WebURL                    *string
	NetShop                                                                  *int16
	ContactName, ContactPhonetic, ContactTel                                 *string
	PriceMin, PriceMax, Base                                                 *string
	MainImages, Category                                                     bool
}

var (
	postCodePattern   = regexp.MustCompile(`^[0-9]{3}-[0-9]{4}$`)
	companyTelPattern = regexp.MustCompile(`^0[0-9]{9,10}$`)
)

func validateRequiredInformation(entity Entity, info requiredInformation) error {
	fields := make([]string, 0)
	require := func(name string, value *string, nonWhitespace bool) {
		if value == nil || *value == "" || (nonWhitespace && strings.TrimSpace(*value) == "") {
			fields = append(fields, name)
		}
	}
	require("company.name", info.CompanyName, true)
	require("company.phonetic", info.CompanyPhonetic, false)
	require("company.representative_name", info.RepresentativeName, true)
	require("company.representative_phonetic", info.RepresentativePhonetic, false)
	if info.PostCode == nil || !postCodePattern.MatchString(*info.PostCode) {
		fields = append(fields, "company.post_code")
	}
	require("company.address", info.Address, false)
	if info.Tel == nil || !companyTelPattern.MatchString(*info.Tel) {
		fields = append(fields, "company.tel")
	}
	require("name", info.Name, false)
	require("phonetic", info.Phonetic, false)
	// JavaScript String.length counts UTF-16 code units, including surrogate pairs.
	if info.Concept == nil || *info.Concept == "" || len(utf16.Encode([]rune(*info.Concept))) > 250 {
		fields = append(fields, "concept")
	}
	if !info.MainImages {
		fields = append(fields, "main_images")
	}
	if !info.Category {
		fields = append(fields, "category")
	}
	if info.TargetMin == nil {
		fields = append(fields, "target_min")
	}
	require("target_max", info.TargetMax, false)
	require("web_url", info.WebURL, false)
	if info.NetShop == nil {
		fields = append(fields, "net_shop")
	}
	require("contact_person_name", info.ContactName, false)
	require("contact_person_phonetic", info.ContactPhonetic, false)
	require("contact_person_tel", info.ContactTel, false)
	if entity == Brand {
		require("price_min", info.PriceMin, false)
		require("price_max", info.PriceMax, false)
		require("base", info.Base, false)
	}
	if len(fields) > 0 {
		return &RequiredFieldsError{Fields: fields}
	}
	return nil
}

type Entity string

const (
	Brand Entity = "brand"
	Shop  Entity = "shop"
)

var (
	ErrInvalidInput = errors.New("invalid submission input")
	ErrNotFound     = errors.New("submission target not found")
	ErrConflict     = errors.New("submission is not allowed in the current state")
)

type State struct {
	ID          int64      `json:"id"`
	SubmittedAt *time.Time `json:"submitted_at"`
	Approval    int16      `json:"approval"`
	Status      int16      `json:"status"`
}

type Repository interface {
	Get(context.Context, Entity, int64, int64) (*State, error)
	Submit(context.Context, Entity, int64, int64) (*State, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func validTarget(entity Entity, id, companyID int64) bool {
	return (entity == Brand || entity == Shop) && id > 0 && companyID > 0
}

func (s *Service) Get(ctx context.Context, entity Entity, id, companyID int64) (*State, error) {
	if !validTarget(entity, id, companyID) {
		return nil, ErrInvalidInput
	}
	return s.repository.Get(ctx, entity, id, companyID)
}

func (s *Service) Submit(ctx context.Context, entity Entity, id, companyID int64) (*State, error) {
	if !validTarget(entity, id, companyID) {
		return nil, ErrInvalidInput
	}
	return s.repository.Submit(ctx, entity, id, companyID)
}
