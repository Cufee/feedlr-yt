package auth

import (
	"os"
	"time"

	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func Middleware(sc *sessions.SessionClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		if os.Getenv("SKIP_AUTH") == "true" {
			c.Locals("session", sessions.Mock(sessions.SessionData{
				UserID:       "clzir9ou400003ipx8kt2rmdq",
				ConnectionID: "c1",
				ExpiresAt:    time.Now().Add(time.Hour * 99999),
			}))
			return c.Next()
		}

		session, err := sc.FromID(c.Cookies("session_id"))
		if err != nil {
			return c.Redirect("/login")
		}

		if session.Valid() {
			c.Locals("session", session)

			session.Refresh()
			sc.Update(session)

			return c.Next()
		}

		return c.Redirect("/login")
	}
}
