package api

import (
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/cufee/tpot/brewed"
)

var ToggleSponsorBlockCategory brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	category := ctx.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(userID, category)
	if err != nil {
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return settings.CategoryToggleButton(category, enabled, false), nil
}

var ToggleSponsorBlock brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	updated, err := logic.ToggleSponsorBlock(userID)
	if err != nil {
		return nil, err
	}
	return settings.SponsorBlockSettings(updated.SponsorBlock), nil
}
