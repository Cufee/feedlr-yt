package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/permissions"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
)

var Admin brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/error?code=404", http.StatusMovedPermanently)
		return nil, nil, nil
	}
	user, err := ctx.Database().GetUser(ctx.Context(), userID)
	if err != nil {
		ctx.Redirect("/error?code=404", http.StatusMovedPermanently)
		return nil, nil, nil
	}

	perms := permissions.Parse(user.Permissions, permissions.Blank)
	if !perms.Has(permissions.ViewAdminPanel) {
		ctx.Redirect("/error?code=404", http.StatusMovedPermanently)
		return nil, nil, nil
	}

	return layouts.App, app.Admin(), nil
}
