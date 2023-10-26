package app

import (
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostAppSettings(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	settings, err := logic.GetUserSettings(userId)
	if err != nil {
		return err
	}

	layout := "layouts/app"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}
	return c.Render(layout, app.Settings(settings))
}
