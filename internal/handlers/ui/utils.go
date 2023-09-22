package ui

import "github.com/gofiber/fiber/v2"

/* If the HX-Request header is set, return a reset layout */
func withLayout(c *fiber.Ctx, layouts ...string) string {
	if c.Request().Header.Peek("HX-Request") != nil {
		return "layouts/app-htmx"
	}
	layout := c.Locals("layout").(string)
	if len(layouts) > 0 {
		layout = layouts[0]
	}
	return layout
}
