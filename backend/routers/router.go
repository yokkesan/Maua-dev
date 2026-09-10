package routers

import (
	"backend/controllers"

	beego "github.com/beego/beego/v2/server/web"
)

func init() {
	ns := beego.NewNamespace("/api",
		beego.NSRouter("/health", &controllers.HealthController{}),
		beego.NSNamespace("/v1",
			beego.NSRouter("/top/notice", &controllers.TopController{}),
		),
	)

	beego.AddNamespace(ns)
}