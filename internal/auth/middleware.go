package auth

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func Middleware(c *fiber.Ctx) error {
	session, err := sessions.FromID(c.Cookies("session_id"))
	if err != nil {
		log.Printf("sessions.FromID: %v\n", err)
		return c.Redirect("/login")
	}

	if uid, valid := session.UserID(); valid {
		c.Locals("userId", uid)
		go session.Refresh()
		return c.Next()
	}

	return c.Redirect("/login")
}
