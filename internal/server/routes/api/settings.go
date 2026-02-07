package api

import (
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/cufee/tpot/brewed"
)

var ToggleSponsorBlockCategory brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("toggle_sponsorblock_category", "unauthorized")
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	category := ctx.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(ctx.Context(), ctx.Database(), userID, category)
	if err != nil {
		metrics.IncUserAction("toggle_sponsorblock_category", "error")
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	metrics.IncUserAction("toggle_sponsorblock_category", "success")
	return settings.CategoryToggleButton(category, enabled, false), nil
}

var ToggleSponsorBlock brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("toggle_sponsorblock", "unauthorized")
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	updated, err := logic.ToggleSponsorBlock(ctx.Context(), ctx.Database(), userID)
	if err != nil {
		metrics.IncUserAction("toggle_sponsorblock", "error")
		return nil, err
	}
	metrics.IncUserAction("toggle_sponsorblock", "success")
	return settings.SponsorBlockSettings(updated.SponsorBlock), nil
}
