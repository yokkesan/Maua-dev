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

	ns := beego.NewNamespace("/api",
		beego.NSRouter(
			"/health",
			&controllers.HealthController{},
		),
		beego.NSNamespace("/v1",
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
		),
	)

	beego.AddNamespace(ns)
}
