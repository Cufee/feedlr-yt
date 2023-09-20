package main

import "github.com/byvko-dev/youtube-app/internal/handlers"

func main() {
	start := handlers.NewServer(3000)
	start()
}
