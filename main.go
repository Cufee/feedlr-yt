package main

import "github.com/byvko-dev/youtube-app/internal/server"

//go:generate go run github.com/steebchen/prisma-client-go generate

func main() {
	// background.StartCronTasks()

	start := server.New()
	start()
}
