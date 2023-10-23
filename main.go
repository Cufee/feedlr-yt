package main

import (
	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
)

//go:generate task db:generate
//go:generate task style:generate

func main() {
	background.StartCronTasks()

	start := server.New()
	start()
}
