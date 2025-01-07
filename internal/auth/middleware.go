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

		if session.Valid() {
			c.Locals("session", session)

			_ = session.Refresh(c.Context())
			return c.Next()
		}

		return c.Redirect("/login")
	}
}
