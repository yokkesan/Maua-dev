package pricescreeningcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/pricescreening"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

type SelectionController struct {
	beego.Controller
	Service *pricescreening.SelectionService
	Table   string
}

func (c *SelectionController) Get() { c.handle(false) }
func (c *SelectionController) Put() { c.handle(true) }

func (c *SelectionController) handle(put bool) {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.fail(http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.fail(http.StatusBadRequest, "invalid id")
		return
	}
	if c.Service == nil {
		c.fail(http.StatusInternalServerError, "internal server error")
		return
	}
	var result *pricescreening.SelectionResult
	if put {
		var input pricescreening.SelectionInput
		decoder := json.NewDecoder(bytes.NewReader(c.Ctx.Input.RequestBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			c.fail(http.StatusBadRequest, "invalid request body")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			c.fail(http.StatusBadRequest, "invalid request body")
			return
		}
		result, err = c.Service.Put(c.Ctx.Request.Context(), c.Table, id, user.CompanyID, input)
	} else {
		result, err = c.Service.Get(c.Ctx.Request.Context(), c.Table, id, user.CompanyID)
	}
	if err != nil {
		switch {
		case errors.Is(err, pricescreening.ErrNotFound):
			c.fail(http.StatusNotFound, "not found")
		case errors.Is(err, pricescreening.ErrInvalidInput):
			c.fail(http.StatusBadRequest, "invalid price screening input")
		default:
			logs.Error("save price screening selection: %v", err)
			c.fail(http.StatusInternalServerError, "internal server error")
		}
		return
	}
	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = result
	_ = c.ServeJSON()
}

func (c *SelectionController) fail(status int, message string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{"error": message}
	_ = c.ServeJSON()
}
