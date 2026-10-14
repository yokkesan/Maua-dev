package authcontroller

import (
	"net/http"
	"os"
	"time"

	"backend/internal/auth"

	beego "github.com/beego/beego/v2/server/web"
)

type LogoutController struct {
	beego.Controller
	Service *auth.Service
}

func (c *LogoutController) Post() {
	if c.Service == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	rawToken := c.Ctx.Input.Cookie("auma_session")

	if err := c.Service.Logout(
		c.Ctx.Request.Context(),
		rawToken,
	); err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	secureCookie := os.Getenv("COOKIE_SECURE") == "true"

	http.SetCookie(
		c.Ctx.ResponseWriter,
		&http.Cookie{
			Name:     "auma_session",
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteLaxMode,
		},
	)

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]string{
		"message": "logged out",
	}

	_ = c.ServeJSON()
}
