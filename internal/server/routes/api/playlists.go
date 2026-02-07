package api

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/tpot/brewed"
)

var ToggleWatchLater brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("toggle_watch_later", "unauthorized")
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	videoID := ctx.Params("id")
	if videoID == "" {
		metrics.IncUserAction("toggle_watch_later", "invalid_request")
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	inWatchLater, err := logic.ToggleWatchLater(ctx.Context(), ctx.Database(), userID, videoID)
	if err != nil {
		metrics.IncUserAction("toggle_watch_later", "error")
		return nil, err
	}

	if ctx.Get("HX-Request") == "" {
		metrics.IncUserAction("toggle_watch_later", "success")
		return nil, ctx.SendStatus(http.StatusOK)
	}

	// Return different button styles based on context
	style := ctx.Query("style")
	switch style {
	case "video":
		metrics.IncUserAction("toggle_watch_later", "success")
		return feed.WatchLaterButton(videoID, inWatchLater, feed.WatchLaterVideo), nil

	case "carousel":
		// For carousel, always removing (carousel only shows pinned items)
		// Fetch video props for OOB card sync
		props, err := logic.GetPlayerPropsWithOpts(ctx.Context(), ctx.Database(), userID, videoID, logic.GetPlayerOptions{WithProgress: true})
		if err != nil {
			return nil, err
		}
		props.Video.InWatchLater = false // Just removed from watch later

		// Check if this was the last item - if so, remove entire section
		count, _ := logic.GetWatchLaterCount(ctx.Context(), ctx.Database(), userID)
		if count == 0 {
			metrics.IncUserAction("toggle_watch_later", "success")
			return feed.SectionRemoveWithCardSync(props.Video, feed.WithProgressActions, feed.WithProgressBar, feed.WithProgressOverlay), nil
		}
		// Otherwise just remove the carousel item + update card
		metrics.IncUserAction("toggle_watch_later", "success")
		return feed.CarouselRemoveWithCardSync(props.Video, feed.WithProgressActions, feed.WithProgressBar, feed.WithProgressOverlay), nil

	case "card":
		// For card style, return the full video card with OOB carousel sync
		props, err := logic.GetPlayerPropsWithOpts(ctx.Context(), ctx.Database(), userID, videoID, logic.GetPlayerOptions{WithProgress: true})
		if err != nil {
			return nil, err
		}
		props.Video.InWatchLater = inWatchLater

		if inWatchLater {
			// Added to watch later - add to carousel (OOB will no-op if section doesn't exist)
			metrics.IncUserAction("toggle_watch_later", "success")
			return feed.VideoCardWithCarouselAdd(props.Video, feed.WithProgressActions, feed.WithProgressBar, feed.WithProgressOverlay), nil
		}
		// Removed from watch later - remove from carousel
		metrics.IncUserAction("toggle_watch_later", "success")
		return feed.VideoCardWithCarouselRemove(props.Video, feed.WithProgressActions, feed.WithProgressBar, feed.WithProgressOverlay), nil

	default:
		metrics.IncUserAction("toggle_watch_later", "success")
		return feed.WatchLaterButton(videoID, inWatchLater, feed.WatchLaterFeed), nil
	}
}
