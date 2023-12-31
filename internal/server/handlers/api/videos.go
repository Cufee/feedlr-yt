package api

import (
	"log"
	"strconv"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/templates/components/feed"
	"github.com/cufee/feedlr-yt/internal/templates/components/shared"
	"github.com/gofiber/fiber/v2"
	"github.com/houseme/mobiledetect/ua"
)

func PostSaveVideoProgress(c *fiber.Ctx) error {
	video := c.Params("id")
	user, _ := c.Locals("userId").(string)
	volume, _ := strconv.Atoi(c.Query("volume"))
	progress, _ := strconv.Atoi(c.Query("progress"))

	err := logic.UpdateViewProgress(user, video, progress)
	if err != nil {
		log.Printf("UpdateViewProgress: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if ua.New(c.Get("User-Agent")).Desktop() {
		// Sound controls don't work on mobile, we always set the volume to 100 there
		err = logic.UpdatePlayerVolume(user, volume)
		if err != nil {
			log.Printf("UpdatePlayerVolume: %v\n", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	if c.Get("HX-Request") == "" {
		return c.SendStatus(fiber.StatusOK)
	}

	props, err := logic.GetPlayerPropsWithOpts(user, video, logic.GetPlayerOptions{WithProgress: true})
	if err != nil {
		log.Printf("GetPlayerPropsWithOpts: %v\n", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.Render("layouts/blank", feed.VideoCard(props.Video, true, true))
}

type videoOpenInput struct {
	Link string `json:"link" form:"link"`
}

func PostVideoOpen(c *fiber.Ctx) error {
	var form videoOpenInput
	err := c.BodyParser(&form)
	if err != nil {
		log.Printf("PostVideoOpen: %v\n", err)
		return c.Render("layouts/blank", shared.OpenVideoInput("", false))
	}
	if form.Link == "" {
		return c.Render("layouts/blank", shared.OpenVideoInput("", true))
	}
	id, valid := logic.VideoIDFromURL(form.Link)
	if !valid {
		return c.Render("layouts/blank", shared.OpenVideoInput(form.Link, false))
	}

	c.Set("HX-Reswap", "none")
	c.Set("HX-Redirect", "/video/"+id)
	return c.SendStatus(fiber.StatusOK)
}
