package brandcontroller

import (
	"net/http"

	"backend/internal/auth"
	"backend/internal/brand"

	beego "github.com/beego/beego/v2/server/web"
)

type BrandController struct {
	beego.Controller
	Service *brand.Service
}

func (c *BrandController) Get() {
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

	brands, err := c.Service.ListByCompanyID(
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
		"brands": brands,
	}
	_ = c.ServeJSON()
}
