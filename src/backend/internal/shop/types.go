package shop

type Shop struct {
	ID               int64  `json:"id"`
	CompanyID        int64  `json:"company_id"`
	Name             string `json:"name"`
	Phonetic         string `json:"phonetic"`
	Tel              string `json:"tel"`
	Email            string `json:"email"`
	RegistrationPlan int16  `json:"registration_plan"`
}
