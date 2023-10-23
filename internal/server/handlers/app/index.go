package app

import (
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostApp(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	subscriptions, err := logic.GetUserSubscriptionsProps(userId)
	if err != nil {
		log.Printf("GetUserSubscriptionsProps: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}
	if len(subscriptions.All) == 0 {
		return c.Redirect("/app/onboarding")
	}

	layout := "layouts/app"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	return c.Render(layout, app.AppHome(*subscriptions))
}
