package main

import (
	"github.com/byvko-dev/youtube-app/internal/logic/background"
	"github.com/byvko-dev/youtube-app/internal/server"
)

func main() {
	go background.StartCronTasks()

	start := server.New(3000)
	start()
}
