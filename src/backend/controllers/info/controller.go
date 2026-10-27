package infocontroller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/internal/adminauth"
	"backend/internal/clientip"
	"backend/internal/httpinput"
	"backend/internal/notification"
	"backend/internal/review"

	beego "github.com/beego/beego/v2/server/web"
)

type Controller struct {
	beego.Controller
	Auth          *adminauth.Service
	Review        *review.Service
	Notifications *notification.Service
	Kind          string
	ClientIP      *clientip.Resolver
}

func (c *Controller) output(code int, v any) {
	c.Ctx.Output.Header("Cache-Control", "no-store")
	c.Ctx.Output.SetStatus(code)
	c.Data["json"] = v
	_ = c.ServeJSON()
}
func (c *Controller) failure(err error) {
	code, message := 500, "internal server error"
	switch {
	case errors.Is(err, review.ErrInput):
		code, message = 400, "invalid review input"
	case errors.Is(err, review.ErrNotFound):
		code, message = 404, "not found"
	case errors.Is(err, review.ErrConflict):
		code, message = 409, "review conflict"
	}
	c.output(code, map[string]string{"error": message})
}
func (c *Controller) Login() {
	if !adminauth.SameOrigin(c.Ctx.Request) {
		c.output(403, map[string]string{"error": "forbidden"})
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if httpinput.Decode(c.Ctx.Request, &input) != nil {
		c.output(400, map[string]string{"error": "invalid request"})
		return
	}
	ip := c.ClientIP.IP(c.Ctx.Request)
	for _, limit := range []struct {
		scope, key string
		count      int
	}{{"info-login-ip", ip, 30}, {"info-login-email", strings.ToLower(strings.TrimSpace(input.Email)), 5}} {
		ok, err := notification.Allow(c.Ctx.Request.Context(), c.Auth.Repository.DB, limit.scope, limit.key, limit.count)
		if err != nil {
			c.output(500, map[string]string{"error": "internal server error"})
			return
		}
		if !ok {
			c.Ctx.Output.Header("Retry-After", "60")
			c.output(429, map[string]string{"error": "too many requests"})
			return
		}
	}
	u, token, expiry, err := c.Auth.Login(c.Ctx.Request.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, adminauth.ErrCredentials) {
			c.output(401, map[string]string{"error": "invalid credentials"})
		} else {
			c.output(500, map[string]string{"error": "internal server error"})
		}
		return
	}
	http.SetCookie(c.Ctx.ResponseWriter, &http.Cookie{Name: adminauth.CookieName, Value: token, Path: "/api/v1/info", Expires: expiry, MaxAge: int(time.Until(expiry).Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	c.output(200, map[string]any{"admin": u})
}
func (c *Controller) Me() {
	u, ok := adminauth.FromContext(c.Ctx.Request.Context())
	if !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	c.output(200, map[string]any{"admin": u})
}
func (c *Controller) Logout() {
	if _, ok := adminauth.FromContext(c.Ctx.Request.Context()); !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	if err := c.Auth.Logout(c.Ctx.Request.Context(), c.Ctx.Input.Cookie(adminauth.CookieName)); err != nil {
		c.output(500, map[string]string{"error": "internal server error"})
		return
	}
	http.SetCookie(c.Ctx.ResponseWriter, &http.Cookie{Name: adminauth.CookieName, Path: "/api/v1/info", MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	c.output(200, map[string]bool{"success": true})
}
func (c *Controller) List() {
	if _, ok := adminauth.FromContext(c.Ctx.Request.Context()); !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	before, err := cursor(c.GetString("before_id"))
	if err != nil {
		c.failure(review.ErrInput)
		return
	}
	result, err := c.Review.List(c.Ctx.Request.Context(), c.Kind, before)
	if err != nil {
		c.failure(err)
		return
	}
	c.output(200, map[string]any{"items": result})
}
func (c *Controller) Detail() {
	if _, ok := adminauth.FromContext(c.Ctx.Request.Context()); !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.failure(review.ErrInput)
		return
	}
	result, err := c.Review.Detail(c.Ctx.Request.Context(), c.Kind, id)
	if err != nil {
		c.failure(err)
		return
	}
	c.output(200, result)
}
func (c *Controller) Decide() {
	u, ok := adminauth.FromContext(c.Ctx.Request.Context())
	if !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	id, err := strconv.ParseInt(c.Ctx.Input.Param(":id"), 10, 64)
	if err != nil || id <= 0 {
		c.failure(review.ErrInput)
		return
	}
	var input struct {
		CompanyID int64 `json:"company_id"`
		Approval  int16 `json:"approval"`
	}
	if httpinput.Decode(c.Ctx.Request, &input) != nil {
		c.failure(review.ErrInput)
		return
	}
	if err := c.Review.Decide(c.Ctx.Request.Context(), c.Kind, id, input.CompanyID, u.ID, input.Approval); err != nil {
		c.failure(err)
		return
	}
	c.output(200, map[string]any{"id": id, "company_id": input.CompanyID, "approval": input.Approval})
}
func (c *Controller) Events() {
	if _, ok := adminauth.FromContext(c.Ctx.Request.Context()); !ok {
		c.output(401, map[string]string{"error": "unauthorized"})
		return
	}
	before, err := cursor(c.GetString("before_id"))
	if err != nil {
		c.failure(review.ErrInput)
		return
	}
	result, err := c.Notifications.List(c.Ctx.Request.Context(), before)
	if err != nil {
		c.failure(err)
		return
	}
	c.output(200, map[string]any{"items": result})
}
func cursor(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, review.ErrInput
	}
	return n, nil
}
