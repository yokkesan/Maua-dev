package main

import (
	"context"
	"log"

	"backend/internal/database"
	_ "backend/routers"

	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	ctx := context.Background()

	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer db.Close()

	if beego.BConfig.RunMode == "dev" {
		beego.BConfig.WebConfig.DirectoryIndex = true
		beego.BConfig.WebConfig.StaticDir["/swagger"] = "swagger"
	}

	beego.Run()
}