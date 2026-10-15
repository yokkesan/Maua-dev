package masters

import (
	internalmasters "backend/internal/registration/masters"
	"net/http"

	beego "github.com/beego/beego/v2/server/web"
)

type Controller struct {
	beego.Controller
	Service *internalmasters.Service
}

func (c *Controller) Get() {
	result, err := c.Service.GetAll(c.Ctx.Request.Context())

	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusInternalServerError)

		c.Data["json"] = map[string]string{
			"error": "failed to fetch registration masters",
		}

		c.ServeJSON()
		return
	}

	c.Data["json"] = result
	c.ServeJSON()
}
