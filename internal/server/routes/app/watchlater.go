package app

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
)

const watchLaterPageSize = 24

var WatchLater brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	page := 1
	if p := ctx.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	offset := (page - 1) * watchLaterPageSize

	videos, hasMore, err := logic.GetWatchLaterVideos(ctx.Context(), ctx.Database(), userID, watchLaterPageSize, offset)
	if err != nil {
		ctx.Err(err)
		return nil, nil, nil
	}

	props := app.WatchLaterPageProps{
		Videos:  videos,
		Page:    page,
		HasMore: hasMore,
	}

	return layouts.App, app.WatchLater(props), nil
}
