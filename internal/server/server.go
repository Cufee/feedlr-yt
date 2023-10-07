package server

import (
	"os"
	"strconv"

	"github.com/byvko-dev/youtube-app/internal/auth"
	apiHandlers "github.com/byvko-dev/youtube-app/internal/server/handlers/api"
	"github.com/byvko-dev/youtube-app/internal/server/handlers/ui"
	"github.com/byvko-dev/youtube-app/internal/sessions"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/template/html/v2"
)

var rootDir = "./static"

func New(port int) func() error {
	portString := strconv.Itoa(port)
	if port == 0 {
		portString = os.Getenv("PORT")
	}

	return func() error {
		store := session.New(session.Config{
			Storage: sessions.Storage,
		})

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

		server.Get("/login", ui.NewLoginHandler(store))
		server.Get("/login/redirect", ui.LoginRedirectHandler)
		server.Get("/login/verify", auth.NewLoginVerifyHandler(store))
		server.Post("/login/start", auth.NewLoginStartHandler(store))

		api := server.Group("/api").Use(auth.NewMiddleware(store))
		api.All("/noop", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		api.Get("/channels/search", apiHandlers.SearchChannelsHandler)
		api.Post("/channels/:id/favorite", apiHandlers.FavoriteChannelHandler)
		api.Post("/channels/:id/subscribe", apiHandlers.SubscribeHandler)
		api.Post("/channels/:id/unsubscribe", apiHandlers.UnsubscribeHandler)

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app").Use(auth.NewMiddleware(store))
		app.Get("/onboarding", ui.OnboardingHandler)
		app.Get("/", ui.AppHandler).Post("/", ui.AppHandler)
		app.Get("/settings", ui.AppSettingsHandler).Post("/settings", ui.AppSettingsHandler)
		api.Get("/watch/:id", apiHandlers.OpenVideoPlayerHandler)

		channels := app.Group("/channels")
		channels.Get("/manage", ui.ManageChannelsAddHandler).Post("/manage", ui.ManageChannelsAddHandler)

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {
			return c.Redirect("/error?code=404&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
