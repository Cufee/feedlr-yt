package api

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/gofiber/fiber/v2"
)

func OpenVideoPlayerHandler(c *fiber.Ctx) error {
	video := c.Params("id")
	props, err := logic.GetVideoByID(video)
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("components/full-screen-player", props, c.Locals("layout").(string))
}
