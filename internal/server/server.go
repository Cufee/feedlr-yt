package server

import (
	"io/fs"
	"net/http"
	"strconv"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	root "github.com/cufee/feedlr-yt/internal/server/routes"
	rapi "github.com/cufee/feedlr-yt/internal/server/routes/api"
	login "github.com/cufee/feedlr-yt/internal/server/routes/api/auth"
	rapp "github.com/cufee/feedlr-yt/internal/server/routes/app"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/cufee/tpot"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/microcosm-cc/bluemonday"
)

func New(db database.Client, ses *sessions.SessionClient, assets fs.FS, policy *bluemonday.Policy, wa *webauthn.WebAuthn, authMw func(c *fiber.Ctx) error, globalMw func(c *fiber.Ctx) error, port ...int) func() error {
	var portString string
	if len(port) > 0 {
		portString = strconv.Itoa(port[0])
	} else {
		portString = utils.MustGetEnv("PORT")
	}

	newCtx := handler.NewBuilder(db, ses, policy, wa)
	toFiber := func(s tpot.Servable[*handler.Context]) func(*fiber.Ctx) error {
		return func(c *fiber.Ctx) error {
			ctx, ok := c.Locals(handler.ContextKeyCustomCtx).(*handler.Context)
			if !ok {
				return adaptor.HTTPHandler(s.Handler(newCtx(c)))(c)
			}
			return adaptor.HTTPHandler(s.Handler(func(w http.ResponseWriter, r *http.Request) *handler.Context { return ctx }))(c)
		}
	}

	return func() error {
		server := fiber.New()
		server.Use(logger.New())
		server.Get("/healthy", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

		// Static files
		server.Use(favicon.New(favicon.Config{
			FileSystem:   http.FS(assets),
			CacheControl: "public, max-age=86400",
			File:         "assets/favicon.png",
			URL:          "/favicon.png",
		}))
		server.Use("/assets", staticWithCacheMiddleware("assets", assets))

		// Maintenance mode
		server.Use(outageMiddleware)

		// Disable caching for all routes
		server.Use(cacheBusterMiddleware)

		// Global middleware (used in dev mode to set up mock auth for all routes)
		if globalMw != nil {
			server.Use(globalMw)
		}

		// Root/Error and etc
		server.All("/", toFiber(root.Landing))
		server.All("/error", toFiber(root.Error))
		server.All("/429", toFiber(root.RateLimited))
		server.Get("/legal/privacy-policy", toFiber(root.PrivacyPolicy))
		server.Get("/legal/terms-of-service", toFiber(root.TermsOfService))
		// Auth/Login
		server.Get("/login", toFiber(root.Login))
		server.Post("/login/begin", toFiber(login.LoginBegin))
		server.Post("/login/finish", toFiber(login.LoginFinish))
		server.Post("/register/begin", toFiber(login.RegistrationBegin))
		server.Post("/register/finish", toFiber(login.RegistrationFinish))

		// Routes with unique auth handlers
		server.Get("/video/:id", toFiber(root.Video))
		server.Get("/channel/:id", toFiber(root.Channel))

		api := server.Group("/api").Use(limiterMiddleware).Use(authMw)
		api.Post("/passkeys/add/begin", toFiber(login.AdditionalPasskeyBegin))
		api.Post("/passkeys/add/finish", toFiber(login.AdditionalPasskeyFinish))
		api.Delete("/passkeys/:passkeyId", toFiber(login.DeletePasskey))

		api.Post("/videos/:id/progress", toFiber(rapi.SaveVideoProgress))
		api.Post("/videos/:id/watch-later", toFiber(rapi.ToggleWatchLater))
		api.Post("/videos/open", toFiber(rapi.OpenVideo))

		api.Get("/channels/search", toFiber(rapi.SearchChannels))
		api.Post("/channels/:id/subscribe", toFiber(rapi.CreateSubscription))
		api.Post("/channels/:id/unsubscribe", toFiber(rapi.RemoveSubscription))
		api.Post("/channels/:id/filter", toFiber(rapi.UpdateVideoFilter))

		api.Post("/settings/sponsorblock", toFiber(rapi.ToggleSponsorBlock))
		api.Post("/settings/sponsorblock/category", toFiber(rapi.ToggleSponsorBlockCategory))

		// All routes used by HTMX should have a POST handler
		app := server.Group("/app").Use(limiterMiddleware).Use(authMw)
		app.All("/", toFiber(rapp.Home))
		app.All("/recent", toFiber(rapp.Recent))
		app.All("/watch-later", toFiber(rapp.WatchLater))
		app.All("/settings", toFiber(rapp.Settings))
		app.All("/onboarding", toFiber(rapp.Onboarding))
		app.All("/subscriptions", toFiber(rapp.Subscriptions))

		// This last handler is a catch-all for any routes that don't exist
		server.Use(func(c *fiber.Ctx) error {
			return c.Redirect("/error?code=404&from=" + c.Path())
		})

		return server.Listen(":" + portString)
	}
}
