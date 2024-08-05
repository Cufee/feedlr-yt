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
	"github.com/houseme/mobiledetect/ua"
)

var Video brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, _ := ctx.Session()

	video := ctx.Params("id")
	// Update cache in background
	go func() {
		c, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		err := logic.CacheVideo(c, ctx.Database(), video)
		if err != nil {
			log.Printf("VideoHandler.UpdateVideoCache error: %v\n", err)
		}
	}()

	if uid, valid := session.UserID(); valid {
		err := ctx.SessionClient().Update(session)
		if err != nil {
			log.Printf("session update error: %s\n", err.Error())
		}

		sctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*250)
		defer cancel()

		settings, err := logic.GetUserSettings(sctx, ctx.Database(), uid)
		if err != nil {
			return nil, nil, ctx.Err(err)
		}

		pctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		defer cancel()

		props, err := logic.GetPlayerPropsWithOpts(pctx, ctx.Database(), uid, video, logic.GetPlayerOptions{WithProgress: true, WithSegments: settings.SponsorBlock.SponsorBlockEnabled})
		if err != nil {
			log.Printf("GetVideoByID: %v", err)
			return nil, nil, ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&feedlr_error=failed to find video", video), http.StatusTemporaryRedirect)
		}

		props.ReportProgress = true
		props.PlayerVolumeLevel = 100
		if ua.New(ctx.Get("User-Agent")).Desktop() {
			props.PlayerVolumeLevel = settings.PlayerVolume // Sound controls don't work on mobile
		}
		if props.Video.Duration > 0 && props.Video.Progress >= props.Video.Duration {
			props.Video.Progress = 0
		}

		props.ReturnURL = ctx.Query("return", "/app")
		return layouts.HeadOnly, pages.Video(props), nil
	}

	// No auth, do not check progress
	props, err := logic.GetPlayerPropsWithOpts(ctx.Context(), ctx.Database(), "", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return nil, nil, ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&feedlr_error=failed to find video", video), http.StatusTemporaryRedirect)
	}

	props.ReturnURL = ctx.Query("return", "/app")
	return layouts.HeadOnly, pages.Video(props), nil
}
