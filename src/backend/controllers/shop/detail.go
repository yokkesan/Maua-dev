package shopcontroller

import (
	"errors"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
)

type ShopDetailController struct {
	beego.Controller
	Service *shop.Service
}

func (c *ShopDetailController) Get() {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.respondError(http.StatusUnauthorized, "unauthorized")
		return
	}

	if c.Service == nil {
		c.respondError(
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	shopID, err := strconv.ParseInt(
		c.Ctx.Input.Param(":id"),
		10,
		64,
	)

	if err != nil || shopID <= 0 {
		c.respondError(http.StatusBadRequest, "invalid shop id")
		return
	}

	result, err := c.Service.GetDetailByID(
		c.Ctx.Request.Context(),
		shopID,
		user.CompanyID,
	)

	if err != nil {
		if errors.Is(err, shop.ErrShopNotFound) {
			c.respondError(http.StatusNotFound, "shop not found")
			return
		}

		c.respondError(
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]any{
		"shop": result,
	}
	_ = c.ServeJSON()
}

func (c *ShopDetailController) respondError(
	status int,
	message string,
) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{
		"error": message,
	}
	_ = c.ServeJSON()
}
