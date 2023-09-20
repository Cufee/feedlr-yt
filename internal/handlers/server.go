package handlers

import (
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

func NewServer(port int) func() error {
	portString := strconv.Itoa(port)
	if port == 0 {
		portString = os.Getenv("PORT")
	}

	engine := html.New("./views", ".html")
	return func() error {
		// Pass the engine to the Views
		app := fiber.New(fiber.Config{
			Views:       engine,
			ViewsLayout: "layouts/main",
		})

		for path, handler := range handlers {
			app.Get(path, handler)
		}

		app.Use(func(c *fiber.Ctx) error {
			return c.Render("error", fiber.Map{
				"message": "Page Not Found",
			})
		})

		return app.Listen(":" + portString)
	}
}
