package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"backend/internal/database"
	"backend/internal/registration"
	"backend/routers"

	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	ctx := context.Background()

	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer db.Close()

	smtpPort, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		log.Fatalf("invalid SMTP_PORT: %v", err)
	}

	mailer := registration.NewSMTPMailer(
		registration.SMTPConfig{
			Host:            os.Getenv("SMTP_HOST"),
			Port:            smtpPort,
			Username:        os.Getenv("SMTP_USERNAME"),
			Password:        os.Getenv("SMTP_PASSWORD"),
			FromAddress:     os.Getenv("SMTP_FROM_ADDRESS"),
			FromName:        os.Getenv("SMTP_FROM_NAME"),
			VerificationURL: os.Getenv("REGISTRATION_VERIFICATION_URL"),
		},
	)

	routers.Register(
		db,
		mailer,
	)

	if beego.BConfig.RunMode == "dev" {
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}

	beego.Run()
}