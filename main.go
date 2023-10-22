package main

import (
	"github.com/byvko-dev/youtube-app/internal/logic/background"
	"github.com/byvko-dev/youtube-app/internal/server"
)

//go:generate task db:generate
//go:generate task style:generate

func main() {
	background.StartCronTasks()

	start := server.New()
	start()
}
