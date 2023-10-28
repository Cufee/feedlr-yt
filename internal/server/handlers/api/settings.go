package api

import (
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/components/settings"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/exp/slices"
)

func PostToggleSponsorBlockCategory(c *fiber.Ctx) error {
	user, _ := c.Locals("userId").(string)
	category := c.Query("category")

	updated, err := logic.ToggleSponsorBlockCategory(user, category)
	if err != nil {
		log.Printf("CountVideoView: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	enabled := slices.Contains(updated.SponsorBlock.SelectedSponsorBlockCategories, category)
	return c.Render("layouts/blank", settings.CategoryToggleButton(category, enabled, false))
}

func PostToggleSponsorBlock(c *fiber.Ctx) error {
	user, _ := c.Locals("userId").(string)
	value := c.Query("value")

	updated, err := logic.ToggleSponsorBlock(user, value == "true")
	if err != nil {
		log.Printf("CountVideoView: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.Render("layouts/blank", settings.SponsorBlockSettings(updated.SponsorBlock))
}
