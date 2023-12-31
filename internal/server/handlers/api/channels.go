package api

import (
	"fmt"
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
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
		log.Print("SearchChannels", err)
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
		return err
	}

	if c.Query("type") == "button" {
		return c.Render("layouts/blank", subscriptions.UnsubscribeButtonSmall(props.ID))
	}
	return c.Render("layouts/blank", subscriptions.SubscribedChannelTile(*props))
}

func UnsubscribeHandler(c *fiber.Ctx) error {
	userId, ok := c.Locals("userId").(string)
	if !ok {
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	channelId := c.Params("id")

	err := logic.DeleteSubscription(userId, channelId)
	if err != nil {
		log.Print("DeleteSubscription", err)
		return err
	}

	if c.Query("type") == "button" {
		return c.Render("layouts/blank", subscriptions.SubscribeButtonSmall(channelId))
	}
	return c.SendStatus(fiber.StatusOK)
}
