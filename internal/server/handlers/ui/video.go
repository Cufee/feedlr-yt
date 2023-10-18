package ui

import (
	"errors"
	"log"

	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/byvko-dev/youtube-app/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func VideoHandler(c *fiber.Ctx) error {
	session, err := sessions.FromID(c.Cookies("session_id"))
	if err != nil && !errors.Is(err, sessions.ErrNotFound) {
		log.Printf("sessions.FromID: %v\n", err)
		return c.Redirect("/login")
	}

	video := c.Params("id")
	if uid, valid := session.UserID(); valid {
		c.Locals("userId", uid)
		go session.Refresh()

		props, err := logic.GetPlayerPropsWithOpts(uid, video, logic.GetPlayerOptions{WithProgress: true, WithSegments: true})
		if err != nil {
			log.Printf("GetVideoByID: %v", err)
			return c.Redirect("/error?message=Something went wrong")
		}
		props.ReportProgress = true

		return c.Render("app/watch", props, withLayout(c, "layouts/with-head"))
	}

	// No auth, do not check progress
	props, err := logic.GetPlayerPropsWithOpts("", video, logic.GetPlayerOptions{WithProgress: false, WithSegments: true})
	if err != nil {
		log.Printf("GetVideoByID: %v", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("share/video", props, withLayout(c, "layouts/with-head"))
}
