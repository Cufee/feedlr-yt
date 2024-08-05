package api

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/tpot/brewed"
	"github.com/houseme/mobiledetect/ua"
)

var SaveVideoProgress brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	video := ctx.Params("id")
	volume, _ := strconv.Atoi(ctx.Query("volume"))
	progress, _ := strconv.Atoi(ctx.Query("progress"))

	err := logic.UpdateViewProgress(ctx.Context(), ctx.Database(), userID, video, progress)
	if err != nil {
		return nil, err
	}

	if ua.New(ctx.Get("User-Agent")).Desktop() {
		// Sound controls don't work on mobile, we always set the volume to 100 there
		err = logic.UpdatePlayerVolume(ctx.Context(), ctx.Database(), userID, volume)
		if err != nil {
			return nil, err
		}
	}

	if ctx.Get("HX-Request") == "" {
		return nil, ctx.SendStatus(http.StatusOK)
	}

	props, err := logic.GetPlayerPropsWithOpts(ctx.Context(), ctx.Database(), userID, video, logic.GetPlayerOptions{WithProgress: true})
	if err != nil {
		return nil, err
	}

	return feed.VideoCard(props.Video, true, true), nil
}

var OpenVideo brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	link, err := ctx.FormValue("link")
	if err != nil {
		return shared.OpenVideoInput("", true), nil
	}
	id, valid := logic.VideoIDFromURL(link)
	if !valid {
		return shared.OpenVideoInput(link, false), nil
	}

	ctx.Set("HX-Reswap", "none")
	return nil, ctx.Redirect("/video/"+id, http.StatusTemporaryRedirect)
}
