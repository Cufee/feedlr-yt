package ui

import (
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

	channels, err := logic.GetUserSubscriptionsProps(userId)
	if err != nil {
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/index", withNavbarProps(c, fiber.Map{
		"Channels": channels,
	}), withLayout(c))
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
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/channels/manage", withNavbarProps(c, fiber.Map{
		"Subscriptions": subscriptions,
	}), withLayout(c))
}

func AppChannelVideoHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	channel := c.Params("channel")
	video := c.Params("video")

	_, _, _ = channel, video, userId

	return c.Render("app/channels/video", withNavbarProps(c, fiber.Map{
		"Title":     "Some Title",
		"ChannelID": "CH555555",
	}), withLayout(c))
}
