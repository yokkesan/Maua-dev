package auth

import "time"

const (
	UserTypeShop  int16 = 1
	UserTypeBrand int16 = 2

	UserRoleManager int16 = 0
	UserRoleGeneral int16 = 1
)

type LoginInput struct {
	Email    string
	Password string
}

type User struct {
	ID           int64
	CompanyID    int64
	Email        string
	PasswordHash string
	UserType     int16
	Role         int16
}

type Session struct {
	ID        int64
	UserID    int64
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type LoginResult struct {
	UserID    int64     `json:"id"`
	CompanyID int64     `json:"company_id"`
	Email     string    `json:"email"`
	UserType  int16     `json:"user_type"`
	Role      int16     `json:"role"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}