package registration

import "time"

const (
	CompanyTypeShop  int16 = 0
	CompanyTypeBrand int16 = 1

	RegistrationPlanBrandFree     int16 = 0
	RegistrationPlanShopPersonal int16 = 10
	RegistrationPlanShopUnit     int16 = 11
)

type VerificationInput struct {
	Email                  string
	CompanyName            string
	CompanyPhonetic        string
	CompanyPostCode        string
	CompanyAddress         string
	CompanyTel             string
	CompanyType            int16
	RepresentativeName     string
	RepresentativePhonetic string
	RegistrationPlan       int16
}

type CreateTokenParams struct {
	VerificationInput
	TokenHash []byte
	ExpiresAt time.Time
}