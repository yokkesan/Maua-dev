package auth

import (
	"net/http"

	beegoctx "github.com/beego/beego/v2/server/web/context"
)

func NewAuthenticationFilter(
	service *Service,
) func(*beegoctx.Context) {
	return func(ctx *beegoctx.Context) {
		rawToken := ctx.Input.Cookie("auma_session")

		user, err := service.Authenticate(
			ctx.Request.Context(),
			rawToken,
		)
		if err != nil {
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

		requestContext := WithUser(
			ctx.Request.Context(),
			user,
		)

		ctx.Request = ctx.Request.WithContext(requestContext)
	}
}
