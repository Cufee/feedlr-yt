package root

import (
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func GetLogin(c *fiber.Ctx) error {
	return c.Render("layouts/HeadOnly", pages.Login())
}
