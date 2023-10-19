package main

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/byvko-dev/youtube-app/internal/templates/pages"
)

//go:generate go run github.com/steebchen/prisma-client-go generate

func main() {
	// background.StartCronTasks()

	// start := server.New()
	// start()

	http.Handle("/", templ.Handler(pages.Landing()))

	fmt.Println("Listening on :3000")
	http.ListenAndServe(":3000", nil)
}
