package root

import (
	"fmt"

	"github.com/byvko-dev/youtube-app/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(c *fiber.Ctx) error {
	message := c.Params("message", c.Query("message", "Something went wrong"))
	code := c.Params("code", c.Query("code", ""))
	from := c.Query("from")

	if code == "404" {
		message = fmt.Sprintf("Page \"%s\" does not exist or was moved.", from)
	}

	return c.Render("layouts/main", pages.Error(message))
}
