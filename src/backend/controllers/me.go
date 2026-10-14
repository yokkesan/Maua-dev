package controllers

import (
	"net/http"

	"backend/internal/auth"

	beego "github.com/beego/beego/v2/server/web"
)

type MeController struct {
	beego.Controller
}

func (c *MeController) Get() {
	user, ok := auth.UserFromContext(
		c.Ctx.Request.Context(),
	)
	if !ok || user == nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]string{
			"error": "unauthorized",
		}
		_ = c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]interface{}{
		"user": map[string]interface{}{
			"id":         user.ID,
			"company_id": user.CompanyID,
			"email":      user.Email,
			"user_type":  user.UserType,
			"role":       user.Role,
		},
	}

	_ = c.ServeJSON()
}
