package server

import (
	"os"
	"strconv"

	apiHandlers "github.com/byvko-dev/youtube-app/internal/server/handlers/api"
	"github.com/byvko-dev/youtube-app/internal/server/handlers/ui"
	"github.com/byvko-dev/youtube-app/internal/server/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

var rootDir = "./static"

func New(port int) func() error {
	portString := strconv.Itoa(port)
	if port == 0 {
		portString = os.Getenv("PORT")
	}

	return func() error {
		server := fiber.New(fiber.Config{
			Views:             html.New(rootDir, ".html"),
			ViewsLayout:       "layouts/main",
			PassLocalsToViews: true,
		})
		server.Use(logger.New())

		server.Static("/static", "./static/served", fiber.Static{
			Compress: true,
		})

		server.Use(func(c *fiber.Ctx) error {
			addRouteLayout(c)
			return c.Next()
		})

		server.Get("/", ui.LandingHandler)
		server.Get("/about", ui.AboutHandler)
		server.Get("/error", ui.ErrorHandler)

		api := server.Group("/api").Use(middleware.AuthMiddleware)
		api.Get("/channels/search", apiHandlers.SearchChannelsHandler)
		api.Post("/channels/:id/favorite", apiHandlers.FavoriteChannelHandler)
		api.Post("/channels/:id/subscribe", apiHandlers.SubscribeHandler)
		api.Post("/channels/:id/unsubscribe", apiHandlers.UnsubscribeHandler)

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app").Use(middleware.AuthMiddleware)
		app.Get("/", ui.AppHandler).Post("/", ui.AppHandler)
		app.Get("/settings", ui.AppSettingsHandler).Post("/settings", ui.AppSettingsHandler)
		app.Get("/watch/:id", ui.AppWatchVideoHandler).Post("/watch/:id", ui.AppWatchVideoHandler)

		channels := app.Group("/channels")
		channels.Get("/manage", ui.ManageChannelsAddHandler).Post("/manage", ui.ManageChannelsAddHandler)

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {
			return c.Redirect("/error?code=404&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
