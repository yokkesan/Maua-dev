package brand

type Brand struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	Name      string `json:"name"`
	Phonetic  string `json:"phonetic"`
	Tel       string `json:"tel"`
	Email     string `json:"email"`
}
