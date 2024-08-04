package api

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/context"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/tpot/brewed"
	"github.com/houseme/mobiledetect/ua"
)

var PostSaveVideoProgress brewed.Partial[*context.Ctx] = func(ctx *context.Ctx) (templ.Component, error) {
	session, ok := ctx.Session()
	if !ok {
		ctx.SetStatus(http.StatusUnauthorized)
		return nil, nil
	}

	video := ctx.PathValue("id")
	volume, _ := strconv.Atoi(ctx.QueryValue("volume"))
	progress, _ := strconv.Atoi(ctx.QueryValue("progress"))

	err := logic.UpdateViewProgress(session.UserID, video, progress)
	if err != nil {
		return nil, err
	}

	if ua.New(ctx.GetHeader("User-Agent")).Desktop() {
		// Sound controls don't work on mobile, we always set the volume to 100 there
		err = logic.UpdatePlayerVolume(session.UserID, volume)
		if err != nil {
			return nil, err
		}
	}

	if ctx.GetHeader("HX-Request") == "" {
		ctx.SetStatus(http.StatusOK)
		return nil, nil
	}

	props, err := logic.GetPlayerPropsWithOpts(session.UserID, video, logic.GetPlayerOptions{WithProgress: true})
	if err != nil {
		return nil, err
	}

	return feed.VideoCard(props.Video, true, true), nil
}

var PostVideoOpen brewed.Partial[*context.Ctx] = func(ctx *context.Ctx) (templ.Component, error) {
	link, err := ctx.FormValue("link")
	if err != nil {
		return shared.OpenVideoInput("", true), nil
	}
	id, valid := logic.VideoIDFromURL(link)
	if !valid {
		return shared.OpenVideoInput(link, false), nil
	}

	ctx.SetHeader("HX-Reswap", "none")
	ctx.Redirect("/video/"+id, http.StatusTemporaryRedirect)
	return nil, nil
}
