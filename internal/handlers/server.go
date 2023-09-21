package handlers

import (
	"os"
	"strconv"

	apiHandlers "github.com/byvko-dev/youtube-app/internal/handlers/api"
	"github.com/byvko-dev/youtube-app/internal/handlers/ui"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

var bgEngine = html.New("./static", ".html")      // Used when a render function is called from props (defaultProps.render)
var defaultEngine = html.New("./static", ".html") // Used by the server

func NewServer(port int) func() error {
	portString := strconv.Itoa(port)
	if port == 0 {
		portString = os.Getenv("PORT")
	}

	return func() error {
		server := fiber.New(fiber.Config{
			Views:             defaultEngine,
			ViewsLayout:       "layouts/main",
			PassLocalsToViews: true,
		})

		server.Static("/static", "./static/served", fiber.Static{
			Compress: true,
		})

		server.Get("/", ui.LandingHandler)
		server.Get("/about", ui.AboutHandler)
		server.Get("/error", ui.ErrorHandler)

		api := server.Group("/api")
		api.Delete("/channels/:id", apiHandlers.DeleteChannelHandler)
		api.Post("/channels/:id/favorite", apiHandlers.FavoriteChannelHandler)

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app")
		app.Get("/", ui.AppHandler).Post("/", ui.AppHandler)
		app.Get("/settings", ui.AppSettingsHandler).Post("/settings", ui.AppSettingsHandler)

		channels := app.Group("/channels")
		channels.Get("/manage", ui.ManageChannelsAddHandler).Post("/manage", ui.ManageChannelsAddHandler)
		channels.Get("/:channel/:video", ui.AppChannelVideoHandler).Post("/:channel/:video", ui.AppChannelVideoHandler)

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {

			return c.Redirect("/error?message=Page Not Found&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
