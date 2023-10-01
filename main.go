package main

import (
	"github.com/byvko-dev/youtube-app/internal/logic/background"
	"github.com/byvko-dev/youtube-app/internal/server"
)

func main() {
	background.CacheAllChannelsWithVideos()

	start := server.New(3000)
	start()
}
