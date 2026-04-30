package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/playlist"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/cufee/tpot/brewed"
)

var CreatePlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	name, err := ctx.FormValue("name")
	if err != nil || name == "" {
		return playlist.CreatePlaylistForm("Playlist name is required"), nil
	}
	description, _ := ctx.FormValue("description")

	p, err := logic.CreateUserPlaylist(ctx.Context(), ctx.Database(), userID, name, description)
	if err != nil {
		return playlist.CreatePlaylistForm("Failed to create playlist"), nil
	}

	return nil, ctx.Redirect(fmt.Sprintf("/app/playlist/%s", p.ID), http.StatusSeeOther)
}

var UpdatePlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	if playlistID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	name, err := ctx.FormValue("name")
	if err != nil || name == "" {
		props, _ := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
		if props != nil {
			return playlist.EditPlaylistForm(props.Playlist, "Playlist name is required"), nil
		}
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}
	description, _ := ctx.FormValue("description")

	err = logic.UpdateUserPlaylist(ctx.Context(), ctx.Database(), userID, playlistID, name, description)
	if err != nil {
		props, _ := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
		if props != nil {
			return playlist.EditPlaylistForm(props.Playlist, "Failed to update playlist"), nil
		}
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	return nil, ctx.Redirect(fmt.Sprintf("/app/playlist/%s", playlistID), http.StatusSeeOther)
}

var DeletePlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	if playlistID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	err := logic.DeleteUserPlaylist(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	return nil, ctx.Redirect("/app/playlists", http.StatusSeeOther)
}

var ImportPlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	link, err := ctx.FormValue("link")
	if err != nil {
		return playlist.ImportPlaylistForm("", false), nil
	}

	ytPlaylistID, valid := logic.PlaylistIDFromURL(link)
	if !valid {
		return playlist.ImportPlaylistForm(link, false), nil
	}

	importCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	newPlaylistID, err := logic.ImportYouTubePlaylist(importCtx, ctx.Database(), userID, ytPlaylistID)
	if err != nil && newPlaylistID == "" {
		return playlist.ImportPlaylistForm(link, false), nil
	}

	return nil, ctx.Redirect(fmt.Sprintf("/app/playlist/%s", newPlaylistID), http.StatusSeeOther)
}

var SyncPlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	if playlistID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := logic.SyncYouTubePlaylist(syncCtx, ctx.Database(), userID, playlistID)
	if err != nil {
		p, findErr := ctx.Database().GetPlaylistByID(ctx.Context(), playlistID)
		if findErr != nil || p.UserID != userID {
			return nil, ctx.SendStatus(http.StatusInternalServerError)
		}
		return shared.RefreshButton(fmt.Sprintf("/api/playlists/%s/sync", playlistID), p.UpdatedAt), nil
	}

	return nil, ctx.Redirect(fmt.Sprintf("/app/playlist/%s", playlistID), http.StatusSeeOther)
}

var AddVideoToPlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	videoID := ctx.Query("videoID")
	playlistID := ctx.Query("playlistID")
	if videoID == "" || playlistID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	// Toggle: remove if already in playlist, add if not
	membership, _ := logic.GetVideoPlaylistMembership(ctx.Context(), ctx.Database(), userID, videoID)
	if membership[playlistID] {
		_ = logic.RemoveVideoFromPlaylist(ctx.Context(), ctx.Database(), userID, playlistID, videoID)
	} else {
		_ = logic.AddVideoToPlaylist(ctx.Context(), ctx.Database(), userID, playlistID, videoID)
	}

	// Return refreshed select
	playlists, _ := logic.GetUserPlaylistsProps(ctx.Context(), ctx.Database(), userID)
	membership, _ = logic.GetVideoPlaylistMembership(ctx.Context(), ctx.Database(), userID, videoID)
	return shared.AddToPlaylistSelect(videoID, playlists, membership), nil
}

var RemoveVideoFromPlaylist brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	videoID := ctx.Params("videoID")
	if playlistID == "" || videoID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	_ = logic.RemoveVideoFromPlaylist(ctx.Context(), ctx.Database(), userID, playlistID, videoID)

	props, err := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	return app.PlaylistVideoFeed(*props), nil
}

var UpdatePlaylistVideoProgress brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	videoID := ctx.Params("videoID")
	if playlistID == "" || videoID == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	hidden := ctx.Query("hidden") == "true"
	progress, _ := strconv.Atoi(ctx.Query("progress"))

	_, _ = logic.UpdateView(ctx.Context(), ctx.Database(), userID, videoID, progress, hidden)

	props, err := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	return app.PlaylistVideoFeed(*props), nil
}

var MovePlaylistItem brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	playlistID := ctx.Params("id")
	videoID := ctx.Params("videoID")
	direction := ctx.Query("direction")
	if playlistID == "" || videoID == "" || direction == "" {
		return nil, ctx.SendStatus(http.StatusBadRequest)
	}

	_ = logic.MovePlaylistItem(ctx.Context(), ctx.Database(), userID, playlistID, videoID, direction)

	props, err := logic.GetPlaylistPageProps(ctx.Context(), ctx.Database(), userID, playlistID)
	if err != nil {
		return nil, ctx.SendStatus(http.StatusInternalServerError)
	}

	return app.PlaylistVideoFeed(*props), nil
}
