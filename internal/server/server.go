package server

import (
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/server/webapi"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/favicon"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func New(db database.Client, ses *sessions.SessionClient, assets fs.FS, wa *webauthn.WebAuthn, globalMw func(c fiber.Ctx) error, port ...int) func() error {
	var portString string
	if len(port) > 0 {
		portString = strconv.Itoa(port[0])
	} else {
		portString = utils.MustGetEnv("PORT")
	}

	return func() error {
		app, closePlayback, err := newApp(db, ses, assets, wa, globalMw)
		if err != nil {
			return err
		}
		defer closePlayback()
		if err := startMetricsServer(); err != nil {
			return err
		}
		return app.Listen(":" + portString)
	}
}

func newApp(db database.Client, ses *sessions.SessionClient, assets fs.FS, wa *webauthn.WebAuthn, globalMw func(c fiber.Ctx) error) (*fiber.App, func(), error) {
	server := fiber.New()
	server.Use(requestMetricsMiddleware)
	server.Use(logger.New())
	server.Get("/healthy", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	frontend, err := newFrontendUI(assets, db, ses)
	if err != nil {
		return nil, nil, err
	}
	server.All("/ui-assets/*", frontend.asset)

	// Static files
	server.Use(favicon.New(favicon.Config{
		FileSystem:   assets,
		CacheControl: "public, max-age=86400",
		File:         "assets/favicon.png",
		URL:          "/favicon.png",
	}))
	server.Use("/assets", staticWithCacheMiddleware("assets", assets))

	// Maintenance mode
	server.Use(frontend.maintenance)

	// Disable caching for all routes
	server.Use(cacheBusterMiddleware)

	// Global middleware (used in dev mode to set up mock auth for all routes)
	if globalMw != nil {
		server.Use(globalMw)
	}

	playbackService := newPlaybackService()
	registerPlaybackRoutes(server, playbackService, db, ses)
	registerMediaAssets(server, db)
	webapi.Register(server, db, ses, wa)

	// Root/Error and etc
	server.Get("/", frontend.page)
	server.Get("/error", frontend.page)
	server.Get("/429", frontend.page)
	server.Get("/legal/privacy-policy", frontend.page)
	server.Get("/legal/terms-of-service", frontend.page)
	// Auth/Login
	server.Get("/login", frontend.page)

	// Routes with unique auth handlers
	server.Get("/video/:id", frontend.page)
	server.Get("/channel/:id", frontend.page)

	// Document navigation always loads the SPA. Authorization belongs to JSON APIs.
	app := server.Group("/app")
	app.Get("/", frontend.page)
	app.Get("/recent", frontend.page)
	app.Get("/watch-later", frontend.page)
	app.Get("/settings", frontend.page)
	app.Get("/settings/:section", frontend.page)
	app.Get("/onboarding", frontend.page)
	app.Get("/subscriptions", frontend.page)
	app.Get("/playlists", frontend.page)
	app.Get("/playlist/:id", frontend.page)

	// This last handler is a catch-all for any routes that don't exist
	server.Use(func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "not_found", "message": "This API endpoint does not exist"}})
		}
		if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
			return c.Status(fiber.StatusMethodNotAllowed).JSON(fiber.Map{"error": fiber.Map{"code": "method_not_allowed", "message": "This document supports GET and HEAD"}})
		}
		c.Status(fiber.StatusNotFound)
		return frontend.page(c)
	})

	return server, playbackService.Close, nil
}

func metricsPath() string {
	path := strings.TrimSpace(os.Getenv("METRICS_PATH"))
	if path == "" {
		path = "/metrics"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func metricsAddress() string {
	port := strings.TrimSpace(os.Getenv("METRICS_PORT"))
	if port == "" {
		port = "9090"
	}

	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

func startMetricsServer() error {
	path := metricsPath()
	addr := metricsAddress()

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	log.Info().Str("addr", listener.Addr().String()).Str("path", path).Msg("metrics server started")

	go func() {
		if serveErr := http.Serve(listener, mux); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Error().Err(serveErr).Msg("metrics server stopped unexpectedly")
		}
	}()

	return nil
}
