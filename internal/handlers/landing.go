package handlers

import "github.com/gofiber/fiber/v2"

func init() {
	newHandler("/", func(c *fiber.Ctx) error {
		return c.Render("landing", nil)
	})
}
