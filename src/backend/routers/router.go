package routers

import (
	"backend/controllers"
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
			&controllers.TopController{},
		),
		beego.NSRouter(
			"/registration/verification",
			&controllers.RegistrationController{
				Service: registrationService,
			},
		),
		beego.NSRouter(
			"/registration/complete",
			&controllers.RegistrationCompleteController{
				Service: registrationService,
			},
		),
		beego.NSRouter(
			"/login",
			&controllers.AuthController{
				Service: authService,
			},
		),
		beego.NSRouter(
			"/me",
			&controllers.MeController{},
		),
	)

	ns := beego.NewNamespace("/api",
		beego.NSRouter(
			"/health",
			&controllers.HealthController{},
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
