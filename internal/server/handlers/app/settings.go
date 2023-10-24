package app

import (
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostAppSettings(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	_ = userId

	layout := "layouts/app"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	return c.Render(layout, app.Settings())
}
