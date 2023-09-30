package main

import (
	"github.com/byvko-dev/youtube-app/internal/server"
)

func main() {
	start := server.New(3000)
	start()
}
