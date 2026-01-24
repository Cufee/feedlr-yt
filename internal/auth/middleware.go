package auth

import (
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func Middleware(sc *sessions.SessionClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		session, err := sc.Get(c.Context(), c.Cookies("session_id"))
		if err != nil {
			return c.Redirect("/login")
		}

		if !session.Valid() {
			return c.Redirect("/login")
		}

		// Check if session has a user associated
		userID, ok := session.UserID()
		if !ok || userID == "" {
			return c.Redirect("/login")
		}

		_ = session.Refresh(c)
		c.Locals("session", session)
		return c.Next()
	}
}
