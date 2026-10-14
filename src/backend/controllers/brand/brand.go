package brandcontroller

import (
	"errors"
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

	if c.Service == nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	result, err := c.Service.GetByCompanyID(
		c.Ctx.Request.Context(),
		user.CompanyID,
	)
	if err != nil {
		if errors.Is(err, brand.ErrBrandNotFound) {
			c.Ctx.Output.SetStatus(http.StatusNotFound)
			c.Data["json"] = map[string]string{
				"error": "brand not found",
			}
		} else {
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
		"brand": result,
	}

	_ = c.ServeJSON()
}
