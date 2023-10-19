package root

import (
	"github.com/byvko-dev/youtube-app/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
)

func LandingHandler(c *fiber.Ctx) error {
	// sessionId := c.Cookies("session_id")
	// if sessionId != "" {
	// 	session, _ := sessions.FromID(sessionId)
	// 	if session.Valid() {
	// 		return c.Redirect("/app")
	// 	}
	// 	session.Delete()
	// 	c.ClearCookie("session_id")
	// }

	return c.Render("layouts/main", pages.Landing())
}
