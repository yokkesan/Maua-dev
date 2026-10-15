package routers

import (
	authcontroller "backend/controllers/auth"
	brandcontroller "backend/controllers/brand"
	healthcontroller "backend/controllers/health"
	registrationcontroller "backend/controllers/registration"
	shopcontroller "backend/controllers/shop"
	topcontroller "backend/controllers/top"
	"backend/internal/auth"
	"backend/internal/brand"
	"backend/internal/registration"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Register(
	db *pgxpool.Pool,
	mailer registration.VerificationMailer,
) {
	registrationRepository := registration.NewPostgresRepository(db)
	registrationService := registration.NewService(
		registrationRepository,
		mailer,
	)

	authRepository := auth.NewPostgresRepository(db)
	authService := auth.NewService(authRepository)

	brandRepository := brand.NewPostgresRepository(db)
	brandService := brand.NewService(brandRepository)

	shopRepository := shop.NewPostgresRepository(db)
	shopService := shop.NewService(shopRepository)

	authFilter := auth.NewAuthenticationFilter(authService)

	brandAuthorizationFilter := auth.NewUserTypeAuthorizationFilter(
		auth.UserTypeBrand,
	)

	shopAuthorizationFilter := auth.NewUserTypeAuthorizationFilter(
		auth.UserTypeShop,
	)

	v1 := beego.NSNamespace(
		"/v1",

		beego.NSRouter(
			"/top/notice",
			&topcontroller.TopController{},
		),

		beego.NSRouter(
			"/registration/verification",
			&registrationcontroller.RegistrationController{
				Service: registrationService,
			},
		),

		beego.NSRouter(
			"/registration/complete",
			&registrationcontroller.RegistrationCompleteController{
				Service: registrationService,
			},
		),

		beego.NSRouter(
			"/login",
			&authcontroller.AuthController{
				Service: authService,
			},
		),

		beego.NSRouter(
			"/logout",
			&authcontroller.LogoutController{
				Service: authService,
			},
		),

		beego.NSRouter(
			"/me",
			&authcontroller.MeController{},
		),

		beego.NSRouter(
			"/brands",
			&brandcontroller.BrandController{
				Service: brandService,
			},
		),

		beego.NSRouter(
			"/shops",
			&shopcontroller.ShopController{
				Service: shopService,
			},
		),
	)

	ns := beego.NewNamespace(
		"/api",

		beego.NSRouter(
			"/health",
			&healthcontroller.HealthController{},
		),

		v1,
	)

	beego.AddNamespace(ns)

	beego.InsertFilter(
		"/api/v1/me",
		beego.BeforeExec,
		authFilter,
	)

	beego.InsertFilter(
		"/api/v1/logout",
		beego.BeforeExec,
		authFilter,
	)

	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		authFilter,
	)

	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		brandAuthorizationFilter,
	)

	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		authFilter,
	)

	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		shopAuthorizationFilter,
	)
}
