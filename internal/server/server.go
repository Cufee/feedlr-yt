package server

import (
	"io/fs"
	"net/http"
	"strconv"

	"github.com/cufee/feedlr-yt/internal/auth"
	root "github.com/cufee/feedlr-yt/internal/server/handlers"
	apiHandlers "github.com/cufee/feedlr-yt/internal/server/handlers/api"
	appHandlers "github.com/cufee/feedlr-yt/internal/server/handlers/app"
	"github.com/cufee/feedlr-yt/internal/server/handlers/channel"
	"github.com/cufee/feedlr-yt/internal/server/handlers/video"
	"github.com/cufee/feedlr-yt/internal/templates"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func New(assets fs.FS, port ...int) func() error {
	var portString string
	if len(port) > 0 {
		portString = strconv.Itoa(port[0])
	} else {
		portString = utils.MustGetEnv("PORT")
	}

	return func() error {
		server := fiber.New(fiber.Config{
			Views:             templates.FiberEngine,
			PassLocalsToViews: true,
		})
		server.Use(logger.New())
		server.Get("/ping", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		// Static files
		server.Use(favicon.New(favicon.Config{
			FileSystem:   http.FS(assets),
			CacheControl: "public, max-age=86400",
			File:         "assets/favicon.ico",
			URL:          "/favicon.ico",
		}))
		server.Use("/assets", staticWithCacheMiddleware("assets", assets))

		// Maintenance mode
		server.Use(outageMiddleware)

		// Disable caching for all routes
		server.Use(cacheBusterMiddleware)

		// Root/Error and etc
		server.Get("/", root.GerOrPosLanding).Post("/", root.GerOrPosLanding)
		server.All("/429", root.RateLimitedHandler)
		server.Get("/error", root.GetOrPostError).Post("/error", root.GetOrPostError)
		// Auth/Login
		server.Get("/login", root.GetLogin)
		server.Get("/login/start", auth.LoginStartHandler)
		server.Get("/login/callback", auth.LoginCallbackHandler)

		// Routes with unique auth handlers
		server.Get("/video/:id", video.VideoHandler)
		server.Get("/channel/:id", channel.ChannelHandler)

		api := server.Group("/api").Use(limiterMiddleware).Use(auth.Middleware)
		api.Post("/videos/:id/progress", apiHandlers.PostSaveVideoProgress)
		api.Post("/videos/open", apiHandlers.PostVideoOpen)

		api.Get("/channels/search", apiHandlers.SearchChannelsHandler)
		api.Post("/channels/:id/subscribe", apiHandlers.SubscribeHandler)
		api.Post("/channels/:id/unsubscribe", apiHandlers.UnsubscribeHandler)

		api.Post("/settings/sponsorblock/category", apiHandlers.PostToggleSponsorBlockCategory)
		api.Post("/settings/sponsorblock", apiHandlers.PostToggleSponsorBlock)

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
