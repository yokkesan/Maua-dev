package brandcontroller

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

type BrandPriceScreeningController struct {
	beego.Controller
	Service *pricescreening.Service
}

func (c *BrandPriceScreeningController) Get() { c.handle(false) }
func (c *BrandPriceScreeningController) Put() { c.handle(true) }

func (c *BrandPriceScreeningController) handle(put bool) {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.screeningError(http.StatusUnauthorized, "unauthorized")
		return
	}
	brandID, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || brandID <= 0 {
		c.screeningError(http.StatusBadRequest, "invalid brand id")
		return
	}
	if c.Service == nil {
		c.screeningError(http.StatusInternalServerError, "internal server error")
		return
	}
	var result *pricescreening.Result
	if put {
		var input struct {
			IDs *[]int64 `json:"price_zone_guideline_ids"`
		}
		decoder := json.NewDecoder(bytes.NewReader(c.Ctx.Input.RequestBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || input.IDs == nil {
			c.screeningError(http.StatusBadRequest, "invalid request body")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			c.screeningError(http.StatusBadRequest, "invalid request body")
			return
		}
		result, err = c.Service.Put(c.Ctx.Request.Context(), brandID, user.CompanyID, *input.IDs)
	} else {
		result, err = c.Service.Get(c.Ctx.Request.Context(), brandID, user.CompanyID)
	}
	if err != nil {
		switch {
		case errors.Is(err, pricescreening.ErrNotFound):
			c.screeningError(http.StatusNotFound, "brand not found")
		case errors.Is(err, pricescreening.ErrInvalidInput):
			c.screeningError(http.StatusBadRequest, "invalid price screening input")
		default:
			logs.Error("price screening failed: %v", err)
			c.screeningError(http.StatusInternalServerError, "internal server error")
		}
		return
	}
	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = result
	_ = c.ServeJSON()
}

func (c *BrandPriceScreeningController) screeningError(status int, message string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{"error": message}
	_ = c.ServeJSON()
}
