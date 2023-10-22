package root

import (
	"github.com/byvko-dev/youtube-app/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func RateLimitedHandler(c *fiber.Ctx) error {
	return c.Render("layouts/main", pages.RateLimited())
}
