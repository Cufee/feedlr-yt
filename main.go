package main

import (
	"context"
	"embed"
	"os"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
	"github.com/cufee/feedlr-yt/internal/sessions"
)

//go:generate templ generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	_, err = db.GetOrCreateUser(context.Background(), "u1")
	if err != nil {
		panic(err)
	}

	_, err = background.StartCronTasks(db)
	if err != nil {
		panic(err)
	}

	ses, err := sessions.New(db)
	if err != nil {
		panic(err)
	}

	start := server.New(db, ses, assetsFs)
	start()
}
