package auth

import (
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v3"
)

func Middleware(sc *sessions.SessionClient) func(c fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		session, err := sc.Get(c.RequestCtx(), c.Cookies("session_id"))
		if err != nil {
			metrics.IncUserEvent("session_lookup", "error")
			metrics.IncUserAction("auth_middleware", "unauthorized")
			return c.Redirect().Status(fiber.StatusFound).To("/login")
		}

		if !session.Valid() {
			metrics.IncUserEvent("session_lookup", "invalid")
			metrics.IncUserAction("auth_middleware", "unauthorized")
			return c.Redirect().Status(fiber.StatusFound).To("/login")
		}

		// Check if session has a user associated
		userID, ok := session.UserID()
		if !ok || userID == "" {
			metrics.IncUserEvent("session_lookup", "missing_user")
			metrics.IncUserAction("auth_middleware", "unauthorized")
			return c.Redirect().Status(fiber.StatusFound).To("/login")
		}

		_ = session.Refresh(c)
		c.Locals("session", session)
		metrics.IncUserEvent("session_lookup", "success")
		metrics.IncUserAction("auth_middleware", "success")
		return c.Next()
	}
}
