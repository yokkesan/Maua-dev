package controllers

import (
	"errors"

	"backend/models"

	beego "github.com/beego/beego/v2/server/web"
)

type TopController struct {
	beego.Controller
}

func (c *TopController) Get() {
	notice, err := models.GetLatestTopNotice(c.Ctx.Request.Context())
	if err != nil {
		if errors.Is(err, models.ErrNoticeNotFound) {
			c.Ctx.Output.SetStatus(404)
			c.Data["json"] = map[string]string{
				"error": "notice not found",
			}
			_ = c.ServeJSON()
			return
		}

		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]string{
			"error": "internal server error",
		}
		_ = c.ServeJSON()
		return
	}

	c.Data["json"] = notice
	_ = c.ServeJSON()
}