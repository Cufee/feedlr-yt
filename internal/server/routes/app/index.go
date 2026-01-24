package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
)

var Home brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	props, err := logic.GetUserVideosProps(ctx.Context(), ctx.Database(), userID)
	if err != nil {
		ctx.Err(err)
		return nil, nil, nil
	}
	if len(props.New) == 0 && len(props.Watched) == 0 {
		ctx.Redirect("/app/onboarding", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	// Fetch watch later videos (limit to 12 for home page carousel)
	watchLater, _, err := logic.GetWatchLaterVideos(ctx.Context(), ctx.Database(), userID, 12, 0)
	if err == nil {
		props.WatchLater = watchLater
	}

	return layouts.App, app.VideosFeed(*props), nil

}
