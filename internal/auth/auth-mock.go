package auth

import (
	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
	"github.com/lucsky/cuid"
)

const mockSessionID = "mock-session-id"

func MockMiddleware(db database.Client) func(c *fiber.Ctx) error {
	sc, err := sessions.New(db)
	if err != nil {
		panic(err)
	}
	us := NewStore(db)

	return func(c *fiber.Ctx) error {
		session, err := sc.Get(c.Context(), c.Cookies("session_id"))
		if err != nil {
			session, err = sc.New(c.Context())
			if err != nil {
				return c.Redirect("/error?message=" + err.Error())
			}
			cookie, err := session.Cookie()
			if err != nil {
				return c.Redirect("/error?message=" + err.Error())
			}
			c.Cookie(cookie)
		}
		if err != nil {
			return c.Redirect("/error?message=" + err.Error())
		}

		user, err := us.FindUser(c.Context(), "mock-user")
		if err != nil {
			user, err = us.NewUser(c.Context(), cuid.New(), "mock-user")
			if err != nil {
				return c.Redirect("/error?message=" + err.Error())
			}
			err = us.SaveUser(c.Context(), &user)
			if err != nil {
				return c.Redirect("/error?message=" + err.Error())
			}
		}

		session, err = session.UpdateUser(c.Context(), null.StringFrom(user.ID), null.StringFrom("mock-user-connection"))
		if err != nil {
			return c.Redirect("/error?message=" + err.Error())
		}

		userID, ok := session.UserID()
		println("session", user.ID, userID, ok)

		c.Locals("session", session)
		return c.Next()
	}
}
