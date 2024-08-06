package root

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Login brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, ok := ctx.Session()
	if ok {
		return nil, nil, ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}
	_, ok = session.UserID()
	if ok {
		return nil, nil, ctx.Redirect("/app", http.StatusTemporaryRedirect)
	}

	return layouts.Main, pages.Login(), nil
}
