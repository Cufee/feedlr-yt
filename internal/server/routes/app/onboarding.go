package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/api/podcastindex"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot/brewed"
)

var Onboarding brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	_, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	discovery, ok := types.ParseMediaSource(ctx.Query("discover", string(types.MediaSourceYouTube)))
	if !ok || discovery == types.MediaSourceAll {
		discovery = types.MediaSourceYouTube
	}

	return layouts.App, app.Onboarding(discovery, podcastindex.DefaultClient != nil), nil
}
