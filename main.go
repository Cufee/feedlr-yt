package main

import (
	"embed"
	"os"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
)

//go:generate task style:generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	background.StartCronTasks(db)

	start := server.New(db, assetsFs)
	start()
}
