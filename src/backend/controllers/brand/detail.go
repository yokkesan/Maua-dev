package brandcontroller

import (
	"errors"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/brand"

	beego "github.com/beego/beego/v2/server/web"
)

type BrandDetailController struct {
	beego.Controller
	Service *brand.Service
}

func (c *BrandDetailController) Get() {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.respondError(http.StatusUnauthorized, "unauthorized")
		return
	}

	if c.Service == nil {
		c.respondError(http.StatusInternalServerError, "internal server error")
		return
	}

	brandID, err := strconv.ParseInt(
		c.Ctx.Input.Param(":id"),
		10,
		64,
	)
	if err != nil || brandID <= 0 {
		c.respondError(http.StatusBadRequest, "invalid brand id")
		return
	}

	result, err := c.Service.GetDetailByID(
		c.Ctx.Request.Context(),
		brandID,
		user.CompanyID,
	)

	if err != nil {
		if errors.Is(err, brand.ErrBrandNotFound) {
			c.respondError(http.StatusNotFound, "brand not found")
			return
		}

		c.respondError(http.StatusInternalServerError, "internal server error")
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]any{
		"brand": result,
	}
	_ = c.ServeJSON()
}

func (c *BrandDetailController) respondError(status int, message string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{
		"error": message,
	}
	_ = c.ServeJSON()
}
