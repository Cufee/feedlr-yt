package api

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/tpot/brewed"
	"github.com/houseme/mobiledetect/ua"
)

var SaveVideoProgress brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("save_video_progress", "unauthorized")
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	video := ctx.Params("id")
	hidden := ctx.Query("hidden") == "true"
	volume, _ := strconv.Atoi(ctx.Query("volume"))
	progress, _ := strconv.Atoi(ctx.Query("progress"))

	resolvedProgress, err := logic.UpdateView(ctx.Context(), ctx.Database(), userID, video, progress, hidden)
	if err != nil {
		metrics.IncUserAction("save_video_progress", "error")
		return nil, err
	}

	// Remove from Watch Later if fully watched
	_ = logic.RemoveFromWatchLaterIfFullyWatched(ctx.Context(), ctx.Database(), userID, video, resolvedProgress)

	if ua.New(ctx.Get("User-Agent")).Desktop() {
		// Sound controls don't work on mobile, we always set the volume to 100 there
		err = logic.UpdatePlayerVolume(ctx.Context(), ctx.Database(), userID, volume)
		if err != nil {
			metrics.IncUserAction("save_video_progress", "error")
			return nil, err
		}
	}

	if ctx.Get("HX-Request") == "" {
		metrics.IncUserAction("save_video_progress", "success")
		return nil, ctx.SendStatus(http.StatusOK)
	}

	props, err := logic.GetPlayerPropsWithOpts(ctx.Context(), ctx.Database(), userID, video, logic.GetPlayerOptions{WithProgress: true})
	if err != nil {
		metrics.IncUserAction("save_video_progress", "error")
		return nil, err
	}

	metrics.IncUserAction("save_video_progress", "success")
	return feed.VideoCard(props.Video, feed.WithProgressActions, feed.WithProgressBar, feed.WithProgressOverlay), nil
}

var OpenVideo brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	link, err := ctx.FormValue("link")
	if err != nil {
		metrics.IncUserAction("open_video", "invalid_request")
		return shared.OpenVideoInput("", true), nil
	}
	id, valid := logic.VideoIDFromURL(link)
	if !valid {
		metrics.IncUserAction("open_video", "invalid_url")
		return shared.OpenVideoInput(link, false), nil
	}

	ctx.Set("HX-Reswap", "none")
	metrics.IncUserAction("open_video", "success")
	return nil, ctx.Redirect("/video/"+id, http.StatusTemporaryRedirect)
}
