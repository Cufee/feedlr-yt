package ui

import "github.com/gofiber/fiber/v2"

func LandingHandler(c *fiber.Ctx) error {
	return c.Render("landing", nil)
}
func ErrorHandler(c *fiber.Ctx) error {
	message := c.Params("message", c.Query("message", "Something went wrong"))
	return c.Render("error", fiber.Map{
		"message": message,
	})
}
