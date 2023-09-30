package api

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/api/youtube"
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
	}, c.Locals("layout").(string))
}

func SubscribeHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id

	return c.SendStatus(fiber.StatusOK)
}

func UnsubscribeHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	_ = id

	return c.SendStatus(fiber.StatusOK)
}

func SearchChannelsHandler(c *fiber.Ctx) error {
	query := c.Query("search")
	channels, err := youtube.Client.SearchChannels(query, 4)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("components/channel-search-tiles", channels, c.Locals("layout").(string))
}
