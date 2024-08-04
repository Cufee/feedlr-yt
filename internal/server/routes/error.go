package root

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/templates/layouts"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/cufee/tpot/brewed"
)

var Error brewed.Page[*handler.Context] = func(ctx *handler.Context) (brewed.Layout[*handler.Context], templ.Component, error) {
	message := ctx.Params("message", ctx.Query("message", "Something went wrong"))
	code := ctx.Params("code", ctx.Query("code", ""))
	from := ctx.Query("from")

	if code == "404" {
		message = fmt.Sprintf("Page \"%s\" does not exist or was moved.", from)
	}
	return layouts.Main, pages.Error(message), nil
}
