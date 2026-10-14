package routers

import (
	authcontroller "backend/controllers/auth"
healthcontroller "backend/controllers/health"
registrationcontroller "backend/controllers/registration"
topcontroller "backend/controllers/top"
	"backend/internal/auth"
	"backend/internal/registration"

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

	authService := auth.NewService(
		authRepository,
	)

	authFilter := auth.NewAuthenticationFilter(
		authService,
	)

	v1 := beego.NSNamespace("/v1",
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
			"/me",
			&authcontroller.MeController{},
		),
	)

	ns := beego.NewNamespace("/api",
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
}
