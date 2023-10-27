package main

import (
	"embed"

	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
)

//go:generate task style:generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	background.StartCronTasks()
	background.CacheAllChannelsWithVideos()

	start := server.New(assetsFs)
	start()
}
