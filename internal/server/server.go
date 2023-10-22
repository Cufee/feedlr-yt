package server

import (
	"os"
	"strconv"

	"github.com/byvko-dev/youtube-app/internal/auth"
	root "github.com/byvko-dev/youtube-app/internal/server/handlers"
	apiHandlers "github.com/byvko-dev/youtube-app/internal/server/handlers/api"
	appHandlers "github.com/byvko-dev/youtube-app/internal/server/handlers/app"
	"github.com/byvko-dev/youtube-app/internal/server/handlers/video"
	"github.com/byvko-dev/youtube-app/internal/templates"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"

	_ "github.com/joho/godotenv/autoload"
)

func New(port ...int) func() error {
	var portString string
	if len(port) > 0 {
		portString = strconv.Itoa(port[0])
	} else {
		portString = os.Getenv("PORT")
	}

	return func() error {
		server := fiber.New(fiber.Config{
			Views:             templates.FiberEngine,
			PassLocalsToViews: true,
		})
		server.Use(logger.New())

		server.Static("/assets", "./assets", fiber.Static{
			Compress: true,
		})

		// Root/Error and etc
		server.Get("/", root.GerOrPosLanding).Post("/", root.GerOrPosLanding)
		server.All("/429", root.RateLimitedHandler)
		server.Get("/error", root.GetOrPostError).Post("/error", root.GetOrPostError)
		// Auth/Login
		server.Get("/login", root.GetLogin)
		server.Get("/login/redirect", root.GetLoginRedirect)
		server.Get("/login/verify", auth.LoginVerifyHandler) // TODO: This should accept a code as fallback
		server.Post("/login/start", auth.LoginStartHandler)

		// Routes with unique auth handlers
		server.Get("/video/:id", video.VideoHandler)

		api := server.Group("/api").Use(limiterMiddleware).Use(auth.Middleware)
		api.Post("/videos/:id/progress", apiHandlers.PostSaveVideoProgress)

		api.Get("/channels/search", apiHandlers.SearchChannelsHandler)
		api.Post("/channels/:id/favorite", apiHandlers.PostFavoriteChannel)
		api.Post("/channels/:id/subscribe", apiHandlers.SubscribeHandler)
		api.Post("/channels/:id/unsubscribe", apiHandlers.UnsubscribeHandler)

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app").Use(limiterMiddleware).Use(auth.Middleware)
		app.Get("/", appHandlers.GetOrPostApp).Post("/", appHandlers.GetOrPostApp)
		app.Get("/settings", appHandlers.GetOrPostAppSettings).Post("/settings", appHandlers.GetOrPostAppSettings)
		app.Get("/onboarding", appHandlers.GetOrPostAppOnboarding).Post("/onboarding", appHandlers.GetOrPostAppOnboarding)
		app.Get("/subscriptions", appHandlers.GetOrPostAppSubscriptions).Post("/subscriptions", appHandlers.GetOrPostAppSubscriptions)

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {
			return c.Redirect("/error?code=404&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
