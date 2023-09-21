package api

import (
	"github.com/gofiber/fiber/v2"
)

func FavoriteChannelHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	currentValue := false

	newValue := true
	if currentValue {
		newValue = false
	}

	return c.Render("components/favorite-channel-button", fiber.Map{
		"ID":        id,
		"Favorited": newValue,
	}, "layouts/reset")
}

func DeleteChannelHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id

	return c.SendStatus(fiber.StatusOK)
}
