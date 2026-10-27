package adminauth

import (
	"net/http"

	beegoctx "github.com/beego/beego/v2/server/web/context"
)

const CookieName = "auma_info_session"

func Filter(s *Service) func(*beegoctx.Context) {
	return func(c *beegoctx.Context) {
		c.Output.Header("Cache-Control", "no-store")
		token := c.Input.Cookie(CookieName)
		if len(token) != 43 {
			c.Output.SetStatus(401)
			c.Output.JSON(map[string]string{"error": "unauthorized"}, false, false)
			return
		}
		u, err := s.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.Output.SetStatus(401)
			c.Output.JSON(map[string]string{"error": "unauthorized"}, false, false)
			return
		}
		// Cookie-authenticated writes require a same-origin request when Origin is present.
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && !SameOrigin(c.Request) {
			c.Output.SetStatus(403)
			c.Output.JSON(map[string]string{"error": "forbidden"}, false, false)
			return
		}
		c.Request = c.Request.WithContext(WithUser(c.Request.Context(), u))
	}
}
