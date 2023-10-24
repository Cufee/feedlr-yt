package root

import (
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func GerOrPosLanding(c *fiber.Ctx) error {
	sessionId := c.Cookies("session_id")
	if sessionId != "" {
		session, _ := sessions.FromID(sessionId)
		if session.Valid() {
			return c.Redirect("/app")
		}
		session.Delete()
		c.ClearCookie("session_id")
	}

	layout := "layouts/main"
	if c.Method() == "POST" || c.Get("HX-Request") != "" {
		layout = "layouts/blank"
	}

	return c.Render(layout, pages.Landing())
}
