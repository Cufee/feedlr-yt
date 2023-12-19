package channel

import (
	"errors"
	"log"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func ChannelHandler(c *fiber.Ctx) error {
	session, err := sessions.FromID(c.Cookies("session_id"))
	if err != nil && !errors.Is(err, sessions.ErrNotFound) {
		log.Printf("sessions.FromID: %v\n", err)
		return c.Redirect("/login")
	}

	if uid, valid := session.UserID(); valid {
		c.Locals("userId", uid)
		go session.Refresh()

		channel, err := logic.GetChannelPageProps(uid, c.Params("id"))
		if err != nil {
			log.Printf("GetChannelPageProps: %v\n", err)
			return c.Redirect("/error?message=Something went wrong")
		}

		return c.Render("layouts/App", pages.Channel(*channel))
	}

	channel, err := logic.GetChannelPageProps("", c.Params("id"))
	if err != nil {
		log.Printf("GetChannelPageProps: %v\n", err)
		return c.Redirect("/error?message=Something went wrong")
	}

	return c.Render("layouts/Share", pages.Channel(*channel))
}
