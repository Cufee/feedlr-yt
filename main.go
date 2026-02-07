//go:build !dev

package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/api/youtube/auth"
	mw "github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/logic/background"
	"github.com/cufee/feedlr-yt/internal/server"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/rs/zerolog/log"

	"github.com/microcosm-cc/bluemonday"
)

//go:generate go tool templ generate

// Embed assets
//
//go:embed assets/*
var assetsFs embed.FS

func main() {
	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	youtubeSync, err := logic.NewYouTubeSyncService(db)
	if err != nil {
		panic(err)
	}
	logic.DefaultYouTubeSync = youtubeSync

	authClient := auth.NewClient(db)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	done, err := authClient.Authenticate(ctx, os.Getenv("YOUTUBE_API_SKIP_AUTH_CACHE") == "true")
	if err != nil {
		cancel()
		panic(err)
	}
	go func() {
		defer cancel()
		<-done
		log.Info().Msg("Youtube API Authenticated")
	}()
	yt, err := youtube.NewClient(os.Getenv("YOUTUBE_API_KEY"), authClient)
	if err != nil {
		panic(err)
	}
	youtube.DefaultClient = yt

	_, err = background.StartCronTasks(db, youtubeSync)
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

	authMiddleware := mw.Middleware(ses)

	start := server.New(db, ses, assetsFs, bluemonday.StrictPolicy(), wa, authMiddleware, nil)
	start()
}
