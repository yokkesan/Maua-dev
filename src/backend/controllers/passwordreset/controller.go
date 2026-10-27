package passwordresetcontroller

import (
	"errors"
	"strings"

	"backend/internal/clientip"
	"backend/internal/httpinput"
	"backend/internal/notification"
	"backend/internal/passwordreset"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

type Controller struct {
	beego.Controller
	Service  *passwordreset.Service
	ClientIP *clientip.Resolver
}

func (c *Controller) output(code int, v any) {
	c.Ctx.Output.Header("Cache-Control", "no-store")
	c.Ctx.Output.SetStatus(code)
	c.Data["json"] = v
	_ = c.ServeJSON()
}
func (c *Controller) Request() {
	var input struct {
		Email string `json:"email"`
	}
	if httpinput.Decode(c.Ctx.Request, &input) != nil {
		c.output(400, map[string]string{"error": "invalid request"})
		return
	}
	email, err := httpinput.Email(input.Email)
	if err != nil {
		c.output(400, map[string]string{"error": "invalid email"})
		return
	}
	ip := c.ClientIP.IP(c.Ctx.Request)
	for _, limit := range []struct {
		scope, key string
		count      int
	}{{"password-reset-ip", ip, 30}, {"password-reset-email", strings.ToLower(email), 3}} {
		ok, err := notification.Allow(c.Ctx.Request.Context(), c.Service.Repository.DB, limit.scope, limit.key, limit.count)
		if err != nil {
			logs.Error("password reset request unavailable")
			c.accepted()
			return
		}
		if !ok {
			c.Ctx.Output.Header("Retry-After", "60")
			c.output(429, map[string]string{"error": "too many requests"})
			return
		}
	}
	if c.Service.Request(c.Ctx.Request.Context(), email) != nil {
		logs.Error("password reset request unavailable")
	}
	c.accepted()
}
func (c *Controller) accepted() {
	c.output(202, map[string]string{"message": "該当するアカウントがある場合、再設定メールを送信します。"})
}
func (c *Controller) Reset() {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if httpinput.Decode(c.Ctx.Request, &input) != nil {
		c.output(400, map[string]string{"error": "invalid request"})
		return
	}
	ip := c.ClientIP.IP(c.Ctx.Request)
	ok, err := notification.Allow(c.Ctx.Request.Context(), c.Service.Repository.DB, "password-reset-confirm", ip, 30)
	if err != nil {
		c.output(500, map[string]string{"error": "internal server error"})
		return
	}
	if !ok {
		c.output(429, map[string]string{"error": "too many requests"})
		return
	}
	err = c.Service.Reset(c.Ctx.Request.Context(), input.Token, input.Password)
	if err != nil {
		code, message := 500, "internal server error"
		if errors.Is(err, passwordreset.ErrInput) || errors.Is(err, passwordreset.ErrToken) {
			code, message = 400, "invalid reset request or expired token"
		}
		c.output(code, map[string]string{"error": message})
		return
	}
	c.output(200, map[string]bool{"success": true})
}
