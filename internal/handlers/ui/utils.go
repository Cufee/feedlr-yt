package ui

import "github.com/gofiber/fiber/v2"

/* If the HX-Request header is set, return a reset layout */
func htmxSkipLayout(c *fiber.Ctx, layouts ...string) string {
	if c.Request().Header.Peek("HX-Request") != nil {
		return "layouts/reset"
	}
	layout := c.App().Config().ViewsLayout
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	return layout
}
