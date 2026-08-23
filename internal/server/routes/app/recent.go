package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot/brewed"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
)

var Recent brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	source, ok := types.ParseMediaSource(ctx.Query("source", string(types.MediaSourceAll)))
	if !ok {
		source = types.MediaSourceAll
	}

	props, err := logic.GetRecentVideosProps(ctx.Context(), ctx.Database(), userID, source)
	if err != nil {
		ctx.Err(err)
		return nil, nil, nil
	}

	return layouts.App, app.History(props, source), nil

}
