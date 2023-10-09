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
	return c.SendStatus(fiber.StatusOK)
}
