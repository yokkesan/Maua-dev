package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/registration"

	beego "github.com/beego/beego/v2/server/web"
)

type RegistrationController struct {
	beego.Controller
	Service *registration.Service
}

type registrationRequest struct {
	Email                  string `json:"email"`
	CompanyName            string `json:"company_name"`
	CompanyPhonetic        string `json:"company_phonetic"`
	CompanyPostCode        string `json:"company_post_code"`
	CompanyAddress         string `json:"company_address"`
	CompanyTel             string `json:"company_tel"`
	CompanyType            int16  `json:"company_type"`
	RepresentativeName     string `json:"representative_name"`
	RepresentativePhonetic string `json:"representative_phonetic"`
	RegistrationPlan       int16  `json:"registration_plan"`
}

func (c *RegistrationController) Post() {
	var request registrationRequest

	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&request); err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]string{
			"error": "invalid request body",
		}
		_ = c.ServeJSON()
		return
	}

	if c.Service == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	result, err := c.Service.CreateVerification(
		c.Ctx.Request.Context(),
		registration.VerificationInput{
			Email:                  request.Email,
			CompanyName:            request.CompanyName,
			CompanyPhonetic:        request.CompanyPhonetic,
			CompanyPostCode:        request.CompanyPostCode,
			CompanyAddress:         request.CompanyAddress,
			CompanyTel:             request.CompanyTel,
			CompanyType:            request.CompanyType,
			RepresentativeName:     request.RepresentativeName,
			RepresentativePhonetic: request.RepresentativePhonetic,
			RegistrationPlan:       request.RegistrationPlan,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, registration.ErrInvalidEmail),
			errors.Is(err, registration.ErrInvalidCompanyType),
			errors.Is(err, registration.ErrInvalidPlan),
			errors.Is(err, registration.ErrCompanyNameRequired),
			errors.Is(err, registration.ErrInputTooLong):

			c.Ctx.Output.SetStatus(http.StatusBadRequest)
			c.Data["json"] = map[string]string{
				"error": err.Error(),
			}

		default:
			c.Ctx.Output.SetStatus(http.StatusInternalServerError)
			c.Data["json"] = map[string]string{
				"error": "internal server error",
			}
		}

		_ = c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(http.StatusCreated)
	c.Data["json"] = map[string]interface{}{
		"message":    "verification token created",
		"token":      result.Token,
		"expires_at": result.ExpiresAt,
	}

	_ = c.ServeJSON()
}

func (c *RegistrationController) Get() {
	if c.Service == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	rawToken := c.GetString("token")

	token, err := c.Service.VerifyToken(
		c.Ctx.Request.Context(),
		rawToken,
	)
	if err != nil {
		switch {
		case errors.Is(err, registration.ErrVerificationTokenInvalid):
			c.Ctx.Output.SetStatus(http.StatusBadRequest)
			c.Data["json"] = map[string]string{
				"error": "invalid verification token",
			}

		case errors.Is(err, registration.ErrVerificationTokenExpired):
			c.Ctx.Output.SetStatus(http.StatusGone)
			c.Data["json"] = map[string]string{
				"error": "verification token has expired",
			}

		case errors.Is(err, registration.ErrVerificationTokenRevoked):
			c.Ctx.Output.SetStatus(http.StatusGone)
			c.Data["json"] = map[string]string{
				"error": "verification token has been revoked",
			}

		case errors.Is(err, registration.ErrVerificationTokenConsumed):
			c.Ctx.Output.SetStatus(http.StatusGone)
			c.Data["json"] = map[string]string{
				"error": "verification token has already been used",
			}

		default:
			c.Ctx.Output.SetStatus(http.StatusInternalServerError)
			c.Data["json"] = map[string]string{
				"error": "internal server error",
			}
		}

		_ = c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]interface{}{
		"message":          "verification token is valid",
		"token_id":         token.ID,
		"email":            token.InvitationEmail,
		"company_name":     token.CompanyName,
		"company_type":     token.CompanyType,
		"registration_plan": token.RegistrationPlan,
		"expires_at":       token.ExpiresAt,
	}

	_ = c.ServeJSON()
}