package shopcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"backend/internal/auth"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
)

type ShopCreateController struct {
	beego.Controller
	Service *shop.Service
}

func (c *ShopCreateController) Post() {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.respondError(http.StatusUnauthorized, "unauthorized")
		return
	}

	if c.Service == nil {
		c.respondError(http.StatusInternalServerError, "internal server error")
		return
	}

	body := bytes.TrimSpace(c.Ctx.Input.RequestBody)
	if len(body) == 0 || body[0] != '{' {
		c.respondError(http.StatusBadRequest, "invalid request body")
		return
	}

	var input shop.CreateShopInput

	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&input); err != nil {
		c.respondError(http.StatusBadRequest, "invalid request body")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		c.respondError(http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := c.Service.Create(
		c.Ctx.Request.Context(),
		user.CompanyID,
		input,
	)

	if err != nil {
		if errors.Is(err, shop.ErrInvalidShopInput) {
			c.respondError(http.StatusBadRequest, "invalid shop input")
			return
		}

		c.respondError(http.StatusInternalServerError, "internal server error")
		return
	}

	c.Ctx.Output.SetStatus(http.StatusCreated)
	c.Data["json"] = map[string]any{"shop": result}
	_ = c.ServeJSON()
}

func (c *ShopCreateController) respondError(status int, message string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{"error": message}
	_ = c.ServeJSON()
}
