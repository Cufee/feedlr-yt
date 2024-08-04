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

	props, err := logic.GetUserVideosProps(userID)
	if err != nil {
		ctx.Err(err)
		return nil, nil, nil
	}
	if len(props.Videos) == 0 {
		ctx.Redirect("/app/onboarding", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	return layouts.App, app.VideosFeed(*props), nil

}
