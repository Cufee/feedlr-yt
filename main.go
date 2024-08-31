package main

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/piped"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/microcosm-cc/bluemonday"
)

//go:generate templ generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	pipedClient, err := piped.NewClient(os.Getenv("PIPED_API_URL"))
	if err != nil {
		panic(err)
	}

	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	_, err = background.StartCronTasks(db, pipedClient)
	if err != nil {
		panic(err)
	}

	ses, err := sessions.New(db)
	if err != nil {
		panic(err)
	}

	host := os.Getenv("COOKIE_DOMAIN")
	origin := fmt.Sprintf("https://%s", host)
	if strings.Contains(origin, "localhost:") {
		origin = fmt.Sprintf("http://%s", host)
	}

	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Feedlr",
		RPID:          strings.Split(host, ":")[0],
		RPOrigins:     []string{origin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 1,
				TimeoutUVD: time.Minute * 1,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Minute * 5,
				TimeoutUVD: time.Minute * 5,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	start := server.New(db, pipedClient, ses, assetsFs, bluemonday.StrictPolicy(), wa)
	start()
}
