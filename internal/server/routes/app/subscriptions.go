package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/api/podcastindex"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot/brewed"
)

var Subscriptions brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		ctx.Redirect("/login", http.StatusTemporaryRedirect)
		return nil, nil, nil
	}

	source, ok := types.ParseMediaSource(ctx.Query("source", string(types.MediaSourceYouTube)))
	if !ok || source == types.MediaSourceAll {
		source = types.MediaSourceYouTube
	}

	channels, err := logic.GetUserSubscribedChannels(ctx.Context(), ctx.Database(), userID)
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	subscriptions := make([]types.ChannelProps, 0, len(channels))
	for _, channel := range channels {
		if channel.MediaSource() == source {
			subscriptions = append(subscriptions, channel)
		}
	}

	return layouts.App, app.Subscriptions(app.SubscriptionPageProps{
		Channels:               subscriptions,
		Source:                 source,
		PodcastSearchAvailable: podcastindex.DefaultClient != nil,
	}), nil
}
