package app

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/byvko-dev/youtube-app/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func GetOrPostAppSubscriptions(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	subscriptions, err := logic.GetUserSubscribedChannels(userId)
	if err != nil {
		log.Printf("GetUserSubscriptionsProps: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	layout := "layouts/app"
	if c.Method() == "POST" {
		layout = "layouts/blank"
	}

	return c.Render(layout, app.Subscriptions(subscriptions))
}
