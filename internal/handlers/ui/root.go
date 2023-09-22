package ui

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func LandingHandler(c *fiber.Ctx) error {
	return c.Render("landing", fiber.Map{
		"NavbarProps": fiber.Map{
			"Hide": true,
		},
	}, withLayout(c))
}

func AboutHandler(c *fiber.Ctx) error {
	return c.Render("about", nil, withLayout(c))
}

func ErrorHandler(c *fiber.Ctx) error {
	message := c.Params("message", c.Query("message", "Something went wrong"))
	code := c.Params("code", c.Query("code", ""))
	from := c.Query("from")

	if code == "404" {
		message = fmt.Sprintf("Page \"%s\" does not exist or was moved.", from)
	}

	return c.Render("error", fiber.Map{
		"message": message,
		"code":    code,
		"from":    from,
	}, withLayout(c))
}
