package app

import (
	"github.com/byvko-dev/youtube-app/internal/templates/pages/app"
	"github.com/gofiber/fiber/v2"
)

func AppHandler(c *fiber.Ctx) error {
	// userId, _ := c.Locals("userId").(string)

	// subscriptions, err := logic.GetUserSubscriptionsProps(userId)
	// if err != nil {
	// 	log.Printf("GetUserSubscriptionsProps: %v", err)
	// 	return c.Redirect("/error?message=Something went wrong")
	// }
	// if len(subscriptions.All) == 0 {
	// 	return c.Redirect("/app/onboarding")
	// }

	// props, err := subscriptions.ToMap()
	// if err != nil {
	// 	log.Printf("UserSubscriptionsFeedProps.ToMap: %v", err)
	// 	return c.Redirect("/error?message=Something went wrong")
	// }

	return c.Render("layouts/main", app.AppHome())
}
