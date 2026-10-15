package shopcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
)

type ShopUpdateController struct {
	beego.Controller
	Service *shop.Service
}

func (c *ShopUpdateController) Patch() {
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

	body := bytes.TrimSpace(c.Ctx.Input.RequestBody)

	if len(body) == 0 || body[0] != '{' {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	var input shop.UpdateShopInput

	decoder := json.NewDecoder(bytes.NewReader(body))

	if err := decoder.Decode(&input); err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	result, err := c.Service.UpdateByID(
		c.Ctx.Request.Context(),
		shopID,
		user.CompanyID,
		input,
	)

	if err != nil {
		switch {
		case errors.Is(err, shop.ErrShopNotFound):
			c.respondError(
				http.StatusNotFound,
				"shop not found",
			)

		case errors.Is(err, shop.ErrInvalidShopInput):
			c.respondError(
				http.StatusBadRequest,
				"invalid shop input",
			)

		default:
			c.respondError(
				http.StatusInternalServerError,
				"internal server error",
			)
		}
		return
	}

	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = map[string]any{
		"shop": result,
	}
	_ = c.ServeJSON()
}

func (c *ShopUpdateController) respondError(
	status int,
	message string,
) {
	c.Ctx.Output.SetStatus(status)

	c.Data["json"] = map[string]string{
		"error": message,
	}

	_ = c.ServeJSON()
}
