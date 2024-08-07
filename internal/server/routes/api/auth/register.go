package auth

import (
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/cufee/tpot/brewed"
)

var RegistrationBegin brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	category := ctx.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(ctx.Context(), ctx.Database(), userID, category)
	if err != nil {
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return settings.CategoryToggleButton(category, enabled, false), nil
}

var RegistrationFinish brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}

	category := ctx.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(ctx.Context(), ctx.Database(), userID, category)
	if err != nil {
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return settings.CategoryToggleButton(category, enabled, false), nil
}
