package api

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/gofiber/fiber/v2"
)

func SearchChannelsHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	query := c.Query("search")

	if len(query) < 5 || len(query) > 32 {
		if len(query) == 0 {
			return c.SendString(``)
		}
		return c.SendString(`<div class="m-auto">Channel name must be between 5 and 32 characters long</div>`)
	}

	channels, err := logic.SearchChannels(userId, query, 4)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("components/search-channels-tiled", channels, c.Locals("layout").(string))
}

func SubscribeHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	channelId := c.Params("id")

	props, err := logic.NewSubscription(userId, channelId)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("components/subs-channels-tile", props, c.Locals("layout").(string))
}

func UnsubscribeHandler(c *fiber.Ctx) error {
	userId, ok := c.Locals("userId").(string)
	if !ok {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	channelId := c.Params("id")

	err := database.C.DeleteSubscription(userId, channelId)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func FavoriteChannelHandler(c *fiber.Ctx) error {
	id := c.Params("id")
	userId, _ := c.Locals("userId").(string)

	updated, err := logic.ToggleSubscriptionIsFavorite(userId, id)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("components/favorite-channel-button", fiber.Map{
		"ID":       id,
		"Favorite": updated,
	}, c.Locals("layout").(string))
}
