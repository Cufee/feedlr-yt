package app

import (
	"github.com/byvko-dev/youtube-app/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostAppOnboarding(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	_ = userId

	layout := "layouts/app"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	return c.Render(layout, app.Onboarding())
}
