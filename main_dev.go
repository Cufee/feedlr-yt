//go:build dev

package main

import (
	"context"
	"os"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/api/youtube/auth"
	mw "github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/rs/zerolog/log"

	"github.com/microcosm-cc/bluemonday"
)

func main() {
	log.Info().Msg("Starting in DEVELOPMENT mode with mock auth")

	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

	youtubeSync, err := logic.NewYouTubeSyncService(db)
	if err != nil {
		panic(err)
	}
	logic.DefaultYouTubeSync = youtubeSync

	youtubeTVSync, err := logic.NewYouTubeTVSyncService(db)
	if err != nil {
		panic(err)
	}
	logic.DefaultYouTubeTVSync = youtubeTVSync

	// YouTube API setup
	authClient, err := auth.NewClient(db)
	if err != nil {
		panic(err)
	}
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
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := youtubeTVSync.RunLifecycleTick(bootCtx); err != nil {
		log.Warn().Err(err).Msg("initial tv sync lifecycle tick failed")
	}
	if err := youtubeTVSync.RunConnectionTick(bootCtx); err != nil {
		log.Warn().Err(err).Msg("initial tv sync connection tick failed")
	}
	bootCancel()

	// Session client (needed by server even though MockMiddleware creates its own)
	ses, err := sessions.New(db)
	if err != nil {
		panic(err)
	}

	// Use mock auth middleware globally - bypasses passkeys for ALL routes
	mockAuth := mw.MockMiddleware(db)

	// Start server with:
	// - os.DirFS(".") for live asset reloading
	// - nil for webauthn (not needed in dev mode)
	// - mockAuth as both auth middleware AND global middleware
	start := server.New(db, ses, os.DirFS("."), bluemonday.StrictPolicy(), nil, mockAuth, mockAuth)
	if err := start(); err != nil {
		panic(err)
	}
}
