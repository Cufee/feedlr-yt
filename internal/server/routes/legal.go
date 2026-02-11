package root

import (
	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/tpot/brewed"
)

var TermsOfService brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	return layouts.Legal, shared.RemoteContentPage("https://byvko-dev.github.io/legal/terms-of-service-partial"), nil
}

var PrivacyPolicy brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	return layouts.Legal, shared.RemoteContentPage("https://byvko-dev.github.io/legal/privacy-policy"), nil
}
