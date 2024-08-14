package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot/brewed"
)

var Settings brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	props, err := logic.GetUserSettings(ctx.Context(), ctx.Database(), userID)
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	passkeys, err := ctx.Database().GetUserPasskeys(ctx.Context(), userID)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, nil, ctx.Err(err)
	}
	for _, pk := range passkeys {
		props.Passkeys = append(props.Passkeys, types.PasskeyToProps(pk))
	}

	return layouts.App, app.Settings(props), nil
}
