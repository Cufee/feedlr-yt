package root

import (
	"log"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Channel brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	userID, _ := ctx.UserID()
	props, err := logic.GetChannelPageProps(ctx.Context(), ctx.Database(), userID, ctx.Params("id"))
	if err != nil {
		return nil, nil, ctx.Err(err)
	}

	if userID != "" {
		subscribed, err := logic.SubscriptionExists(ctx.Context(), ctx.Database(), userID, props.Channel.ID)
		if err != nil {
			log.Printf("FindSubscription: %v\n", err)
		}
		props.Subscribed = subscribed
	}

	return layouts.App, pages.Channel(*props), nil
}
