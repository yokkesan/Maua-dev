package auth

import (
	"net/http"

	beegoctx "github.com/beego/beego/v2/server/web/context"
)

func NewUserTypeAuthorizationFilter(
	requiredUserType int16,
) func(*beegoctx.Context) {
	return func(ctx *beegoctx.Context) {
		user, ok := UserFromContext(
			ctx.Request.Context(),
		)
		if !ok || user == nil {
			ctx.Output.SetStatus(http.StatusUnauthorized)
			ctx.Output.JSON(
				map[string]string{
					"error": "unauthorized",
				},
				false,
				false,
			)
			return
		}

		if user.UserType != requiredUserType {
			ctx.Output.SetStatus(http.StatusForbidden)
			ctx.Output.JSON(
				map[string]string{
					"error": "forbidden",
				},
				false,
				false,
			)
			return
		}
	}
}
