package routers

import (
	"os"

	authcontroller "backend/controllers/auth"
	brandcontroller "backend/controllers/brand"
	healthcontroller "backend/controllers/health"
	pricescreeningcontroller "backend/controllers/pricescreening"
	registrationcontroller "backend/controllers/registration"
	registrationmasterscontroller "backend/controllers/registration/masters"
	submissioncontroller "backend/controllers/registrationsubmission"
	shopcontroller "backend/controllers/shop"
	topcontroller "backend/controllers/top"

	"backend/internal/auth"
	"backend/internal/brand"
	"backend/internal/pricescreening"
	"backend/internal/registration"
	registrationmasters "backend/internal/registration/masters"
	"backend/internal/registrationsubmission"
	"backend/internal/shop"
	"backend/internal/storage"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Register(
	db *pgxpool.Pool,
	mailer registration.VerificationMailer,
) {
	// multipart本文全体を12MiBに制限する。画像2枚（各5MB）と登録JSONを含む。
	// Controller実行前のBeegoによるmultipart解析にも適用する。
	beego.BConfig.MaxUploadSize = 12 << 20

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
	priceScreeningService := pricescreening.NewService(pricescreening.NewPostgresRepository(db))
	selectionService := pricescreening.NewSelectionService(db)

	// =========================================================
	// Image Storage
	// =========================================================
	var imageService *storage.MainImageService

	storageRoot := os.Getenv("STORAGE_LOCAL_ROOT")

	if storageRoot != "" {
		localStorage, err := storage.NewLocalStorage(storageRoot)
		if err != nil {
			panic("initialize local storage: " + err.Error())
		}

		imageService, err = storage.NewMainImageService(localStorage)
		if err != nil {
			panic("initialize main image service: " + err.Error())
		}
	}

	// =========================================================
	// Shop
	// =========================================================
	shopRepository := shop.NewPostgresRepository(db)
	shopService := shop.NewService(shopRepository)
	submissionService := registrationsubmission.NewService(
		registrationsubmission.NewPostgresRepository(db),
	)

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
		// Registration Masters
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

		// ブランド一覧取得
		beego.NSRouter(
			"/brands",
			&brandcontroller.BrandController{
				Service: brandService,
			},
			"get:Get",
		),

		// ブランド新規追加
		beego.NSRouter(
			"/brands",
			&brandcontroller.BrandCreateController{
				Service:      brandService,
				ImageService: imageService,
			},
			"post:Post",
		),

		// プライススクリーニングの保存・再取得
		beego.NSRouter(
			"/brands/:id/price-screening",
			&brandcontroller.BrandPriceScreeningController{Service: priceScreeningService},
			"put:Put;get:Get",
		),

		// ブランドのプライススクリーニング選択保存・取得
		beego.NSRouter(
			"/brands/:id/price-screening/selection",
			&pricescreeningcontroller.SelectionController{Service: selectionService, Table: "brands"},
			"put:Put;get:Get",
		),

		// ブランド登録申請・申請状態取得
		beego.NSRouter(
			"/brands/:id/submission",
			&submissioncontroller.Controller{Service: submissionService, Entity: registrationsubmission.Brand},
			"post:Post;get:Get",
		),

		// ブランド詳細取得
		beego.NSRouter(
			"/brands/:id",
			&brandcontroller.BrandDetailController{
				Service: brandService,
			},
			"get:Get",
		),

		// ブランド詳細更新
		beego.NSRouter(
			"/brands/:id",
			&brandcontroller.BrandUpdateController{
				Service: brandService,
			},
			"patch:Patch",
		),

		// -----------------------------------------------------
		// Shop
		// -----------------------------------------------------

		// ショップ一覧取得
		beego.NSRouter(
			"/shops",
			&shopcontroller.ShopController{
				Service: shopService,
			},
			"get:Get",
		),

		// ショップ新規追加
		beego.NSRouter(
			"/shops",
			&shopcontroller.ShopCreateController{
				Service:      shopService,
				ImageService: imageService,
			},
			"post:Post",
		),

		// ショップのプライススクリーニング選択保存・取得
		beego.NSRouter(
			"/shops/:id/price-screening/selection",
			&pricescreeningcontroller.SelectionController{Service: selectionService, Table: "shops"},
			"put:Put;get:Get",
		),

		// ショップ登録申請・申請状態取得
		beego.NSRouter(
			"/shops/:id/submission",
			&submissioncontroller.Controller{Service: submissionService, Entity: registrationsubmission.Shop},
			"post:Post;get:Get",
		),

		// ショップ詳細取得
		beego.NSRouter(
			"/shops/:id",
			&shopcontroller.ShopDetailController{
				Service: shopService,
			},
			"get:Get",
		),

		// ショップ詳細更新
		beego.NSRouter(
			"/shops/:id",
			&shopcontroller.ShopUpdateController{
				Service: shopService,
			},
			"patch:Patch",
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

	// =========================================================
	// Brand Authorization
	// =========================================================

	// ブランド一覧・新規追加：認証
	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		authFilter,
	)

	// ブランド一覧・新規追加：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/brands",
		beego.BeforeExec,
		brandAuthorizationFilter,
	)

	// プライススクリーニング：認証・ユーザー種別による認可
	beego.InsertFilter("/api/v1/brands/:id/price-screening", beego.BeforeExec, authFilter)
	beego.InsertFilter("/api/v1/brands/:id/price-screening", beego.BeforeExec, brandAuthorizationFilter)
	beego.InsertFilter("/api/v1/brands/:id/price-screening/selection", beego.BeforeExec, authFilter)
	beego.InsertFilter("/api/v1/brands/:id/price-screening/selection", beego.BeforeExec, brandAuthorizationFilter)

	// ブランド登録申請・申請状態取得：認証・種別認可
	beego.InsertFilter("/api/v1/brands/:id/submission", beego.BeforeExec, authFilter)
	beego.InsertFilter("/api/v1/brands/:id/submission", beego.BeforeExec, brandAuthorizationFilter)

	// ブランド詳細：認証
	beego.InsertFilter(
		"/api/v1/brands/:id",
		beego.BeforeExec,
		authFilter,
	)

	// ブランド詳細：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/brands/:id",
		beego.BeforeExec,
		brandAuthorizationFilter,
	)

	// =========================================================
	// Shop Authorization
	// =========================================================

	// ショップ一覧・新規追加：認証
	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		authFilter,
	)

	// ショップ一覧・新規追加：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/shops",
		beego.BeforeExec,
		shopAuthorizationFilter,
	)

	// ショップ登録申請・申請状態取得：認証・種別認可
	beego.InsertFilter("/api/v1/shops/:id/submission", beego.BeforeExec, authFilter)
	beego.InsertFilter("/api/v1/shops/:id/submission", beego.BeforeExec, shopAuthorizationFilter)
	beego.InsertFilter("/api/v1/shops/:id/price-screening/selection", beego.BeforeExec, authFilter)
	beego.InsertFilter("/api/v1/shops/:id/price-screening/selection", beego.BeforeExec, shopAuthorizationFilter)

	// ショップ詳細：認証
	beego.InsertFilter(
		"/api/v1/shops/:id",
		beego.BeforeExec,
		authFilter,
	)

	// ショップ詳細：ユーザー種別による認可
	beego.InsertFilter(
		"/api/v1/shops/:id",
		beego.BeforeExec,
		shopAuthorizationFilter,
	)
}
