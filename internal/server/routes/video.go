package root

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot/brewed"
)

var Video brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, _ := ctx.Session()

	video := ctx.Params("id")
	// Refresh cache in the background if stale
	go func(id string) {
		c, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()
		logic.RefreshVideoCache(c, ctx.Database(), id)
	}(video)

	if uid, valid := session.UserID(); valid {
		sctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*250)
		defer cancel()

		settings, err := logic.GetUserSettings(sctx, ctx.Database(), uid)
		if err != nil {
			return nil, nil, ctx.Err(err)
		}

		pctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1500)
		defer cancel()

		props, err := logic.GetPlayerPropsWithOpts(pctx, ctx.Database(), uid, video, logic.GetPlayerOptions{WithProgress: true, WithSegments: settings.SponsorBlock.SponsorBlockEnabled})
		if err != nil {
			return nil, nil, videoFetchFallback(ctx, video, err)
		}

		props.ReportProgress = true
		if props.Video.Duration > 0 && props.Video.Progress >= props.Video.Duration {
			props.Video.Progress = 0
		}

		// Check if video is in watch later
		inWatchLater, _ := logic.IsInWatchLater(pctx, ctx.Database(), uid, video)
		props.Video.InWatchLater = inWatchLater

		// Populate user playlists for "add to playlist" dropdown
		userPlaylists, _ := logic.GetUserPlaylistsProps(pctx, ctx.Database(), uid)
		props.UserPlaylists = userPlaylists
		if len(userPlaylists) > 0 {
			videoInPlaylists, _ := logic.GetVideoPlaylistMembership(pctx, ctx.Database(), uid, video)
			props.VideoInPlaylistIDs = videoInPlaylists
		}

		props.ReturnURL = ctx.Query("return", "/app")
		return playerLayout(props)
	}

	// No auth, do not check progress
	pctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1500)
	defer cancel()

	props, err := logic.GetPlayerPropsWithOpts(pctx, ctx.Database(), "", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		return nil, nil, videoFetchFallback(ctx, video, err)
	}

	props.ReturnURL = ctx.Query("return", "/app")
	return playerLayout(props)
}

// playerLayout picks the audio player page for podcast episodes.
func playerLayout(props types.VideoPlayerProps) (brewed.Layout[*handler.Context], templ.Component, error) {
	if props.Video.IsPodcast() {
		return layouts.Video(props.Video), pages.Podcast(props), nil
	}
	return layouts.Video(props.Video), pages.Video(props), nil
}

// videoFetchFallback keeps the YouTube fallback for videos, while podcast
// episodes surface a local error page instead.
func videoFetchFallback(ctx *handler.Context, video string, err error) error {
	if strings.HasPrefix(video, "pe_") {
		return ctx.Err(err)
	}
	return ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&from=feedler.app", video), http.StatusTemporaryRedirect)
}
