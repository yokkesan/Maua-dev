package shopcontroller

import (
	"errors"
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
	user, ok := auth.UserFromContext(
		c.Ctx.Request.Context(),
	)
	if !ok || user == nil {
		c.Ctx.Output.SetStatus(
			http.StatusUnauthorized,
		)
		c.Data["json"] = map[string]string{
			"error": "unauthorized",
		}
		_ = c.ServeJSON()
		return
	}

	if c.Service == nil {
		c.Ctx.Output.SetStatus(
			http.StatusInternalServerError,
		)
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

	if errors.Is(err, shop.ErrShopNotFound) {
		c.Ctx.Output.SetStatus(
			http.StatusNotFound,
		)
		c.Data["json"] = map[string]string{
			"error": "shop not found",
		}
		_ = c.ServeJSON()
		return
	}

	if err != nil {
		c.Ctx.Output.SetStatus(
			http.StatusInternalServerError,
		)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	c.Ctx.Output.SetStatus(
		http.StatusOK,
	)
	c.Data["json"] = map[string]any{
		"shop": result,
	}
	_ = c.ServeJSON()
}
