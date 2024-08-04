package root

import (
	"fmt"
	"log"
	"net/http"

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
		err := logic.CacheVideo(video)
		if err != nil {
			log.Printf("VideoHandler.UpdateVideoCache error: %v\n", err)
		}
	}()

	if uid, valid := session.UserID(); valid {
		// c.Locals("userId", uid)
		// go session.Refresh()

		settings, err := logic.GetUserSettings(uid)
		if err != nil {
			return nil, nil, ctx.Err(err)
		}

		props, err := logic.GetPlayerPropsWithOpts(uid, video, logic.GetPlayerOptions{WithProgress: true, WithSegments: settings.SponsorBlock.SponsorBlockEnabled})
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
	props, err := logic.GetPlayerPropsWithOpts("", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return nil, nil, ctx.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&feedlr_error=failed to find video", video), http.StatusTemporaryRedirect)
	}

	props.ReturnURL = ctx.Query("return", "/app")
	return layouts.HeadOnly, pages.Video(props), nil
}
