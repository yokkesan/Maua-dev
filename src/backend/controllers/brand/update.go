package brandcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/brand"

	beego "github.com/beego/beego/v2/server/web"
)

type BrandUpdateController struct {
	beego.Controller
	Service *brand.Service
}

func (c *BrandUpdateController) Patch() {
	user, ok := auth.UserFromContext(
		c.Ctx.Request.Context(),
	)

	if !ok || user == nil {
		c.respondError(
			http.StatusUnauthorized,
			"unauthorized",
		)
		return
	}

	if c.Service == nil {
		c.respondError(
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	brandID, err := strconv.ParseInt(
		c.Ctx.Input.Param(":id"),
		10,
		64,
	)

	if err != nil || brandID <= 0 {
		c.respondError(
			http.StatusBadRequest,
			"invalid brand id",
		)
		return
	}

	// リクエストはJSONオブジェクトのみ許可する。
	body := bytes.TrimSpace(
		c.Ctx.Input.RequestBody,
	)

	if len(body) == 0 || body[0] != '{' {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	var input brand.UpdateBrandInput

	decoder := json.NewDecoder(
		bytes.NewReader(body),
	)

	if err := decoder.Decode(&input); err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	// 2つ目のJSON値など、余分なデータを拒否する。
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
		brandID,
		user.CompanyID,
		input,
	)

	if err != nil {
		switch {
		case errors.Is(
			err,
			brand.ErrBrandNotFound,
		):
			c.respondError(
				http.StatusNotFound,
				"brand not found",
			)

		case errors.Is(
			err,
			brand.ErrInvalidBrandInput,
		):
			c.respondError(
				http.StatusBadRequest,
				"invalid brand input",
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
		"brand": result,
	}

	_ = c.ServeJSON()
}

func (c *BrandUpdateController) respondError(
	status int,
	message string,
) {
	c.Ctx.Output.SetStatus(status)

	c.Data["json"] = map[string]string{
		"error": message,
	}

	_ = c.ServeJSON()
}
