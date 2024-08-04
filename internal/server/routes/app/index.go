package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/context"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/cufee/tpot/brewed"
)

var GetOrPostApp brewed.Page[*context.Ctx] = func(ctx *context.Ctx) (brewed.Layout[*context.Ctx], templ.Component, error) {
	session, ok := ctx.Session()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	props, err := logic.GetUserVideosProps(session.UserID)
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
