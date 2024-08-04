package root

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Landing brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	session, ok := ctx.Session()
	if !session.Valid() {
		ctx.ClearCookie("session_id")
	}
	if !ok {
		return layouts.Main, pages.Landing(), nil
	}
	return nil, nil, ctx.Redirect("/app", http.StatusTemporaryRedirect)
}
