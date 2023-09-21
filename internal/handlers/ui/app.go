package ui

import (
	"github.com/byvko-dev/youtube-app/internal/api"
	"github.com/gofiber/fiber/v2"
)

func withAppNavbarProps(c *fiber.Ctx, other ...fiber.Map) fiber.Map {
	props := fiber.Map{
		"AppNavbarProps": fiber.Map{
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
	channels, err := api.GetUserChannels("1")
	if err != nil {
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("app/index", withAppNavbarProps(c, fiber.Map{
		"Channels": channels,
	}), htmxSkipLayout(c))
}

func AppSettingsHandler(c *fiber.Ctx) error {
	return c.Render("app/settings", withAppNavbarProps(c), htmxSkipLayout(c))
}

func ManageChannelsAddHandler(c *fiber.Ctx) error {
	return c.Render("app/channels/manage", withAppNavbarProps(c), htmxSkipLayout(c))
}

func AppChannelVideoHandler(c *fiber.Ctx) error {
	channel := c.Params("channel")
	video := c.Params("video")

	_, _ = channel, video

	return c.Render("app/channels/video", withAppNavbarProps(c, fiber.Map{
		"Title":     "Some Title",
		"ChannelID": "CH555555",
	}), htmxSkipLayout(c))
}
