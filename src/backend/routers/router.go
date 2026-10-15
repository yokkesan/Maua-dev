package routers

import (
	authcontroller "backend/controllers/auth"
	brandcontroller "backend/controllers/brand"
	healthcontroller "backend/controllers/health"
	registrationcontroller "backend/controllers/registration"
	registrationmasterscontroller "backend/controllers/registration/masters"
	shopcontroller "backend/controllers/shop"
	topcontroller "backend/controllers/top"

	"backend/internal/auth"
	"backend/internal/brand"
	"backend/internal/registration"
	registrationmasters "backend/internal/registration/masters"
	"backend/internal/shop"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Register(
	db *pgxpool.Pool,
	mailer registration.VerificationMailer,
) {
	// =========================================================
	// Registration
	// =========================================================
	registrationRepository := registration.NewPostgresRepository(db)
	registrationService := registration.NewService(
		registrationRepository,
		mailer,
	)

	// =========================================================
	// Registration Masters
	// =========================================================
	mastersRepository := registrationmasters.NewRepository(db)
	mastersService := registrationmasters.NewService(mastersRepository)

	// =========================================================
	// Auth
	// =========================================================
	authRepository := auth.NewPostgresRepository(db)
	authService := auth.NewService(authRepository)

	// =========================================================
	// Brand
	// =========================================================
	brandRepository := brand.NewPostgresRepository(db)
	brandService := brand.NewService(brandRepository)

	// =========================================================
	// Shop
	// =========================================================
	shopRepository := shop.NewPostgresRepository(db)
	shopService := shop.NewService(shopRepository)

	// =========================================================
	// Authentication / Authorization Filters
	// =========================================================
	authFilter := auth.NewAuthenticationFilter(authService)

	brandAuthorizationFilter := auth.NewUserTypeAuthorizationFilter(
		auth.UserTypeBrand,
	)

	shopAuthorizationFilter := auth.NewUserTypeAuthorizationFilter(
		auth.UserTypeShop,
	)

	// =========================================================
	// API v1
	// =========================================================
	v1 := beego.NSNamespace(
		"/v1",

		// -----------------------------------------------------
		// Top
		// -----------------------------------------------------
		beego.NSRouter(
			"/top/notice",
			&topcontroller.TopController{},
		),

		// -----------------------------------------------------
		// Registration
		// -----------------------------------------------------
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

		// -----------------------------------------------------
		// Registration Masters（追加）
		// -----------------------------------------------------
		beego.NSRouter(
			"/registration/masters",
			&registrationmasterscontroller.Controller{
				Service: mastersService,
			},
		),

		// -----------------------------------------------------
		// Authentication
		// -----------------------------------------------------
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

		// -----------------------------------------------------
		// Brand
		// -----------------------------------------------------
		beego.NSRouter(
			"/brands",
			&brandcontroller.BrandController{
				Service: brandService,
			},
		),

		// -----------------------------------------------------
		// Shop
		// -----------------------------------------------------
		beego.NSRouter(
			"/shops",
			&shopcontroller.ShopController{
				Service: shopService,
			},
		),
	)

	// =========================================================
	// API Namespace
	// =========================================================
	ns := beego.NewNamespace(
		"/api",

		beego.NSRouter(
			"/health",
			&healthcontroller.HealthController{},
		),

		v1,
	)

	beego.AddNamespace(ns)

	// =========================================================
	// Authentication Filters
	// =========================================================

	// ログインユーザー取得
	beego.InsertFilter(
		"/api/v1/me",
		beego.BeforeExec,
		authFilter,
	)

	// ログアウト
	beego.InsertFilter(
		"/api/v1/logout",
		beego.BeforeExec,
		authFilter,
	)

	// =========================================================
	// Brand Authorization
	// =========================================================

	// ブランド：認証
	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		authFilter,
	)

	// ブランド：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		brandAuthorizationFilter,
	)

	// =========================================================
	// Shop Authorization
	// =========================================================

	// ショップ：認証
	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		authFilter,
	)

	// ショップ：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		shopAuthorizationFilter,
	)
}
