package root

import (
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func GerOrPosLanding(c *fiber.Ctx) error {
	layout := "layouts/main"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	return c.Render(layout, pages.Landing())
}
