package server

import (
	"os"
	"strconv"

	"github.com/byvko-dev/youtube-app/internal/auth"
	apiHandlers "github.com/byvko-dev/youtube-app/internal/server/handlers/api"
	"github.com/byvko-dev/youtube-app/internal/server/handlers/ui"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

var rootDir = "./static"

func New(port ...int) func() error {
	var portString string
	if len(port) > 0 {
		portString = strconv.Itoa(port[0])
	}
	portString = os.Getenv("PORT")

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

		// Root/Error and etc
		server.Get("/", ui.LandingHandler)
		server.Get("/error", ui.ErrorHandler)
		// Auth/Login
		server.Get("/login", ui.LoginHandler)
		server.Get("/login/redirect", ui.LoginRedirectHandler)
		server.Get("/login/verify", auth.LoginVerifyHandler)
		// server.Post("/login/verify", auth.LoginVerifyHandler) TODO: This should accept a code as fallback
		server.Post("/login/start", auth.LoginStartHandler)

		// Routes with unique auth handlers
		server.Get("/video/:id", ui.VideoHandler)

		api := server.Group("/api").Use(limiterMiddleware).Use(auth.Middleware)
		api.All("/noop", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		api.Post("/videos/:id/progress", apiHandlers.SaveVideoProgressHandler)

		api.Get("/channels/search", apiHandlers.SearchChannelsHandler)
		api.Post("/channels/:id/favorite", apiHandlers.FavoriteChannelHandler)
		api.Post("/channels/:id/subscribe", apiHandlers.SubscribeHandler)
		api.Post("/channels/:id/unsubscribe", apiHandlers.UnsubscribeHandler)

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app").Use(limiterMiddleware).Use(auth.Middleware)
		app.Get("/", ui.AppHandler).Post("/", ui.AppHandler)
		app.Get("/onboarding", ui.OnboardingHandler)
		app.Get("/settings", ui.AppSettingsHandler).Post("/settings", ui.AppSettingsHandler)

		channels := app.Group("/channels")
		channels.Get("/manage", ui.ManageChannelsAddHandler).Post("/manage", ui.ManageChannelsAddHandler)

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {
			return c.Redirect("/error?code=404&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
