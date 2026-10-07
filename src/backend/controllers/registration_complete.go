package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/registration"

	beego "github.com/beego/beego/v2/server/web"
)

type RegistrationCompleteController struct {
	beego.Controller
	Service *registration.Service
}

type registrationCompleteRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (c *RegistrationCompleteController) Post() {
	if c.Service == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	var request registrationCompleteRequest

	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&request); err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadRequest)
		c.Data["json"] = map[string]string{
			"error": "invalid request body",
		}
		_ = c.ServeJSON()
		return
	}

	result, err := c.Service.CompleteRegistration(
		c.Ctx.Request.Context(),
		request.Token,
		request.Password,
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

		case errors.Is(err, registration.ErrPasswordRequired),
			errors.Is(err, registration.ErrPasswordTooLong):

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

	response := map[string]interface{}{
		"message":    "registration completed",
		"company_id": result.CompanyID,
		"user_id":    result.UserID,
	}

	if result.BrandID != nil {
		response["brand_id"] = *result.BrandID
	}

	if result.ShopID != nil {
		response["shop_id"] = *result.ShopID
	}

	c.Ctx.Output.SetStatus(http.StatusCreated)
	c.Data["json"] = response

	_ = c.ServeJSON()
}