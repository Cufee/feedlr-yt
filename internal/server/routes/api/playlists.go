package api

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/tpot/brewed"
)

var ToggleWatchLater brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	videoID := ctx.Params("id")
	if videoID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	inWatchLater, err := logic.ToggleWatchLater(ctx.Context(), ctx.Database(), userID, videoID)
	if err != nil {
		return nil, err
	}

	if ctx.Get("HX-Request") == "" {
		return nil, ctx.SendStatus(http.StatusOK)
	}

	// Return different button styles based on context
	style := ctx.Query("style")
	switch style {
	case "video":
		return feed.WatchLaterButton(videoID, inWatchLater, feed.WatchLaterVideo), nil
	case "carousel":
		// For carousel, remove the entire item when unpinned
		if !inWatchLater {
			return feed.WatchLaterRemoved(videoID), nil
		}
		return feed.WatchLaterButton(videoID, inWatchLater, feed.WatchLaterCarousel), nil
	default:
		return feed.WatchLaterButton(videoID, inWatchLater, feed.WatchLaterFeed), nil
	}
}
