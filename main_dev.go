//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/api/youtube/auth"
	mw "github.com/cufee/feedlr-yt/internal/auth"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/rs/zerolog/log"

	"github.com/microcosm-cc/bluemonday"
)

//go:generate go tool templ generate

func main() {
	db, err := database.NewSQLiteClient(os.Getenv("DATABASE_PATH"))
	if err != nil {
		panic(err)
	}

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

	ses, err := sessions.New(db)
	if err != nil {
		panic(err)
	}

	host := os.Getenv("COOKIE_DOMAIN")
	origin := fmt.Sprintf("https://%s", host)
	if strings.Contains(origin, "localhost:") {
		origin = fmt.Sprintf("http://%s", host)
	}

	authMiddleware := mw.MockMiddleware(db)

	start := server.New(db, ses, os.DirFS("."), bluemonday.StrictPolicy(), nil, authMiddleware)
	start()
}
