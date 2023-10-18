package ui

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/gofiber/fiber/v2"
)

func withNavbarProps(c *fiber.Ctx, other ...fiber.Map) fiber.Map {
	props := fiber.Map{
		"NavbarProps": fiber.Map{
			"BackURL":    c.Query("from"),
			"CurrentURL": c.Path(),
			"AddFromQuery": func(to, from string) string {
				if to == from {
					return ""
				}
				return "?from=" + from
			},
		},
	}

	if len(other) > 0 {
		for k, v := range other[0] {
			props[k] = v
		}
	}
	return props
}

func AppHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	subscriptions, err := logic.GetUserSubscriptionsProps(userId)
	if err != nil {
		log.Printf("GetUserSubscriptionsProps: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}
	if len(subscriptions.All) == 0 {
		return c.Redirect("/app/onboarding")
	}

	props, err := subscriptions.ToMap()
	if err != nil {
		log.Printf("UserSubscriptionsFeedProps.ToMap: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/index", withNavbarProps(c, props), withLayout(c))
}

func AppSettingsHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	_ = userId

	return c.Render("app/settings", withNavbarProps(c), withLayout(c))
}

func ManageChannelsAddHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)

	subscriptions, err := logic.GetUserSubscriptionsProps(userId)
	if err != nil {
		log.Printf("GetUserSubscriptionsProps: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/channels/manage", withNavbarProps(c, fiber.Map{
		"Subscriptions": subscriptions,
	}), withLayout(c))
}

func AppWatchVideoHandler(c *fiber.Ctx) error {
	video := c.Params("id")
	user, _ := c.Locals("userId").(string)

	props, err := logic.GetVideoWithOptions(user, video, logic.GetVideoOptions{WithProgress: true, WithSegments: true})
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/watch", props, withLayout(c, "layouts/with-head"))
}

func OnboardingHandler(c *fiber.Ctx) error {
	return c.Render("app/onboarding", withNavbarProps(c), withLayout(c))
}
