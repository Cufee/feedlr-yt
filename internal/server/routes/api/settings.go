package api

import (
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/context"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/cufee/tpot/brewed"
)

var PostToggleSponsorBlockCategory brewed.Partial[*context.Ctx] = func(ctx *context.Ctx) (templ.Component, error) {
	session, ok := ctx.Session()
	if !ok {
		ctx.SetStatus(http.StatusUnauthorized)
		return nil, nil
	}

	category := ctx.QueryValue("category")

	updated, err := logic.ToggleSponsorBlockCategory(session.UserID, category)
	if err != nil {
		return nil, err
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return settings.CategoryToggleButton(category, enabled, false), nil
}

var PostToggleSponsorBlock brewed.Partial[*context.Ctx] = func(ctx *context.Ctx) (templ.Component, error) {
	session, ok := ctx.Session()
	if !ok {
		ctx.SetStatus(http.StatusUnauthorized)
		return nil, nil
	}

	updated, err := logic.ToggleSponsorBlock(session.UserID)
	if err != nil {
		return nil, err
	}
	return settings.SponsorBlockSettings(updated.SponsorBlock), nil
}
