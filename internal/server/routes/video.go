package root

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Video brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, _ := ctx.Session()

	video := ctx.Params("id")
	// Update cache in the background
	go func(id string) {
		c, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()

		err := logic.UpdateChannelVideoCache(c, ctx.Database(), id)
		if err != nil {
			log.Printf("VideoHandler.UpdateVideoCache error: %v\n", err)
		}
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
			return nil, nil, ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&from=feedler.app", video), http.StatusTemporaryRedirect)
		}

		props.ReportProgress = true
		if props.Video.Duration > 0 && props.Video.Progress >= props.Video.Duration {
			props.Video.Progress = 0
		}

		// Check if video is in watch later
		inWatchLater, _ := logic.IsInWatchLater(pctx, ctx.Database(), uid, video)
		props.Video.InWatchLater = inWatchLater

		props.ReturnURL = ctx.Query("return", "/app")
		return layouts.Video(props.Video), pages.Video(props), nil
	}

	// No auth, do not check progress
	pctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*1500)
	defer cancel()

	props, err := logic.GetPlayerPropsWithOpts(pctx, ctx.Database(), "", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		return nil, nil, ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&from=feedler.app", video), http.StatusTemporaryRedirect)
	}

	props.ReturnURL = ctx.Query("return", "/app")
	return layouts.Video(props.Video), pages.Video(props), nil
}
