package app

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"

	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
)

var PlaylistsIndex brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	playlists, err := logic.GetUserPlaylistsProps(ctx.Context(), ctx.Database(), userID)
	if err != nil {
		ctx.Err(err)
		return nil, nil, nil
	}

	props := app.PlaylistsPageProps{
		Playlists: playlists,
	}

	return layouts.App, app.PlaylistsIndex(props), nil
}

var PlaylistDetail brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	playlistID := ctx.Params("id")
	if playlistID == "" {
		ctx.Redirect("/app/playlists", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	props, err := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		ctx.Redirect("/app/playlists", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	return layouts.App, app.PlaylistDetail(*props), nil
}

var PlaylistFeed brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	if playlistID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	props, err := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}

	return app.PlaylistVideoFeed(*props), nil
}
