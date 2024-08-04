package auth

import (
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func Middleware(s *sessions.SessionClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		session, err := s.FromID(c.Cookies("session_id"))
		if err != nil {
			return c.Redirect("/login")
		}

		if session.Valid() {
			c.Locals("session", session)

			session.Refresh()
			go s.Update(session)

			return c.Next()
		}

		return c.Redirect("/login")
	}
}
