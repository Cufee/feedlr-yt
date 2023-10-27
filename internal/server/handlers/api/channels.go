package api

import (
	"fmt"
	"log"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/feedlr-yt/internal/templates/components/subscriptions"
	"github.com/gofiber/fiber/v2"
)

func SearchChannelsHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	query := c.Query("search")

	if len(query) < 5 || len(query) > 32 {
		if len(query) == 0 {
			return c.SendString(``)
		}
		return c.SendString(`<div class="m-auto text-2xl">Channel name must be between 5 and 32 characters long</div>`)
	}

	channels, err := logic.SearchChannels(userId, query, 4)
	if err != nil {
		log.Print(err)
		return err
	}
	if len(channels) == 0 {
		return c.SendString(fmt.Sprintf(`<div class="m-auto text-2xl">Didn't find any channels named <span class="font-bold">%s</span></div>`, query))
	}

	return c.Render("layouts/blank", subscriptions.SearchResultChannels(channels))
}

func SubscribeHandler(c *fiber.Ctx) error {
	userId, _ := c.Locals("userId").(string)
	channelId := c.Params("id")

	props, err := logic.NewSubscription(userId, channelId)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("layouts/blank", subscriptions.SubscribedChannelTile(*props))
}

func UnsubscribeHandler(c *fiber.Ctx) error {
	userId, ok := c.Locals("userId").(string)
	if !ok {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	channelId := c.Params("id")

	err := database.DefaultClient.DeleteSubscription(userId, channelId)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func PostFavoriteChannel(c *fiber.Ctx) error {
	id := c.Params("id")
	userId, _ := c.Locals("userId").(string)

	updated, err := logic.ToggleSubscriptionIsFavorite(userId, id)
	if err != nil {
		log.Print(err)
		return err
	}

	return c.Render("layouts/blank", feed.ChannelFavoriteButton(id, updated))
}
