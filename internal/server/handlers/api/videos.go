package api

import (
	"log"
	"strconv"

	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/gofiber/fiber/v2"
)

func SaveVideoProgressHandler(c *fiber.Ctx) error {
	video := c.Params("id")
	user, _ := c.Locals("userId").(string)
	progress, _ := strconv.Atoi(c.Query("progress"))

	err := logic.UpdateViewProgress(user, video, progress)
	if err != nil {
		log.Printf("CountVideoView: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if c.Get("HX-Request") == "" {
		return c.SendStatus(fiber.StatusOK)
	}

	props, err := logic.GetVideoWithProgress(user, video)
	if err != nil {
		log.Printf("GetVideoWithProgress: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Render("components/video-tile", props, c.Locals("layout").(string))
}
