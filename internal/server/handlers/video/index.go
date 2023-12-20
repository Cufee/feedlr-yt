package video

import (
	"errors"
	"fmt"
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
	"github.com/houseme/mobiledetect/ua"
)

func VideoHandler(c *fiber.Ctx) error {
	session, err := sessions.FromID(c.Cookies("session_id"))
	if err != nil && !errors.Is(err, sessions.ErrNotFound) {
		log.Printf("sessions.FromID: %v\n", err)
		return c.Redirect("/login")
	}

	video := c.Params("id")
	// Update cache in background
	go func() {
		err := logic.UpdateVideoCache(video)
		if err != nil {
			log.Printf("VideoHandler.UpdateVideoCache error: %v\n", err)
		}
	}()

	if uid, valid := session.UserID(); valid {
		c.Locals("userId", uid)
		go session.Refresh()

		settings, err := logic.GetUserSettings(uid)
		if err != nil {
			log.Printf("GetUserSettings: %v\n", err)
			return c.Redirect("/error?message=Something went wrong")
		}

		props, err := logic.GetPlayerPropsWithOpts(uid, video, logic.GetPlayerOptions{WithProgress: true, WithSegments: settings.SponsorBlock.SponsorBlockEnabled})
		if err != nil {
			log.Printf("GetVideoByID: %v", err)
			return c.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&feedlr_error=failed to find video", video))
		}

		props.ReportProgress = true
		props.PlayerVolumeLevel = 100
		if ua.New(c.Get("User-Agent")).Desktop() {
			props.PlayerVolumeLevel = settings.PlayerVolume // Sound controls don't work on mobile
		}
		if props.Video.Duration > 0 && props.Video.Progress >= props.Video.Duration {
			props.Video.Progress = 0
		}

		props.ReturnURL = c.Query("return", "/app")
		return c.Render("layouts/HeadOnly", pages.Video(props))
	}

	// No auth, do not check progress
	props, err := logic.GetPlayerPropsWithOpts("", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return c.Redirect(fmt.Sprintf("https://www.youtube.com/watch?v=%s&feedlr_error=failed to find video", video))
	}

	props.ReturnURL = c.Query("return", "/app")
	return c.Render("layouts/HeadOnly", pages.Video(props))
}
