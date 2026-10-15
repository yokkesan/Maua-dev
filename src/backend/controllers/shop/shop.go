package shopcontroller

import (
	"net/http"

	"backend/internal/auth"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
)

type ShopController struct {
	beego.Controller
	Service *shop.Service
}

func (c *ShopController) Get() {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())

	if !ok || user == nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.Data["json"] = map[string]string{
			"error": "unauthorized",
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

	shops, err := c.Service.ListByCompanyID(
		c.Ctx.Request.Context(),
		user.CompanyID,
	)

	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]any{
		"shops": shops,
	}
	_ = c.ServeJSON()
}
