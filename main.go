package main

import (
	"embed"

	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
)

//go:generate task db:generate
//go:generate task style:generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	background.StartCronTasks()

	start := server.New(assetsFs)
	start()
}
