package registrationsubmissioncontroller

import (
	"errors"
	"net/http"
	"strconv"

	"backend/internal/auth"
	"backend/internal/registrationsubmission"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

type Controller struct {
	beego.Controller
	Service *registrationsubmission.Service
	Entity  registrationsubmission.Entity
}

func (c *Controller) Get()  { c.respond(false) }
func (c *Controller) Post() { c.respond(true) }

func (c *Controller) respond(submit bool) {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.respondError(http.StatusUnauthorized, "unauthorized")
		return
	}
	if c.Service == nil {
		c.respondError(http.StatusInternalServerError, "internal server error")
		return
	}
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.respondError(http.StatusBadRequest, "invalid id")
		return
	}
	var state *registrationsubmission.State
	if submit {
		state, err = c.Service.Submit(c.Ctx.Request.Context(), c.Entity, id, user.CompanyID)
	} else {
		state, err = c.Service.Get(c.Ctx.Request.Context(), c.Entity, id, user.CompanyID)
	}
	if err != nil {
		var required *registrationsubmission.RequiredFieldsError
		switch {
		case errors.Is(err, registrationsubmission.ErrNotFound):
			c.respondError(http.StatusNotFound, "not found")
		case errors.Is(err, registrationsubmission.ErrConflict):
			c.respondError(http.StatusConflict, "submission conflict")
		case errors.Is(err, registrationsubmission.ErrInvalidInput):
			c.respondError(http.StatusBadRequest, "invalid submission input")
		case errors.As(err, &required):
			c.Ctx.Output.SetStatus(http.StatusUnprocessableEntity)
			c.Data["json"] = map[string]any{"error": required.Error(), "fields": required.Fields}
			_ = c.ServeJSON()
		default:
			logs.Error("registration submission failed: %v", err)
			c.respondError(http.StatusInternalServerError, "internal server error")
		}
		return
	}
	c.Ctx.Output.SetStatus(http.StatusOK)
	c.Data["json"] = state
	_ = c.ServeJSON()
}

func (c *Controller) respondError(status int, message string) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{"error": message}
	_ = c.ServeJSON()
}
