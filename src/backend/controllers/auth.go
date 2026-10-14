package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"backend/internal/auth"

	beego "github.com/beego/beego/v2/server/web"
)

type AuthController struct {
	beego.Controller
	Service *auth.Service
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *AuthController) Post() {
	var request loginRequest

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

	result, err := c.Service.Login(
		c.Ctx.Request.Context(),
		auth.LoginInput{
			Email:    request.Email,
			Password: request.Password,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail),
			errors.Is(err, auth.ErrPasswordRequired):

			c.Ctx.Output.SetStatus(http.StatusBadRequest)
			c.Data["json"] = map[string]string{
				"error": err.Error(),
			}

		case errors.Is(err, auth.ErrInvalidCredentials):
			c.Ctx.Output.SetStatus(http.StatusUnauthorized)
			c.Data["json"] = map[string]string{
				"error": "invalid credentials",
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

	secureCookie := os.Getenv("COOKIE_SECURE") == "true"

	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(
		c.Ctx.ResponseWriter,
		&http.Cookie{
			Name:     "auma_session",
			Value:    result.Token,
			Path:     "/",
			Expires:  result.ExpiresAt,
			MaxAge:   maxAge,
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteLaxMode,
		},
	)

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]interface{}{
		"user": map[string]interface{}{
			"id":         result.UserID,
			"company_id": result.CompanyID,
			"email":      result.Email,
			"user_type":  result.UserType,
			"role":       result.Role,
		},
	}

	_ = c.ServeJSON()
}
