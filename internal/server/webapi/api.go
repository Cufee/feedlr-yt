// Package webapi provides the versioned JSON boundary for the application UI.
package webapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/url"
	"strconv"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/rs/zerolog/log"
)

type API struct {
	db       database.Client
	sessions *sessions.SessionClient
	wa       *webauthn.WebAuthn
}
type apiError struct {
	status        int
	code, message string
	fields        map[string]string
}

func (e *apiError) Error() string { return e.message }
func fail(status int, code, message string) error {
	return &apiError{status: status, code: code, message: message}
}
func invalid(field, message string) error {
	return &apiError{status: 422, code: "validation", message: "Check the submitted values", fields: map[string]string{field: message}}
}

func errorJSON(c fiber.Ctx, status int, code, message string, fields map[string]string) error {
	return c.Status(status).JSON(Error{Error: Problem{Code: code, Message: message, Fields: fields}})
}

func (a *API) handle(fn fiber.Handler, protected bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		if protected && viewer(c) == "" {
			return errorJSON(c, 401, "unauthenticated", "Please sign in to continue", nil)
		}
		err := fn(c)
		if err == nil {
			return nil
		}
		var problem *apiError
		if errors.As(err, &problem) {
			return errorJSON(c, problem.status, problem.code, problem.message, problem.fields)
		}
		if database.IsErrNotFound(err) {
			return errorJSON(c, 404, "not_found", "This item is unavailable", nil)
		}
		log.Error().Err(err).Str("path", c.Path()).Msg("frontend API request failed")
		return errorJSON(c, 500, "internal_error", "The request could not be completed. Please try again.", nil)
	}
}

// Register mounts the SPA's JSON API and document-level OAuth callbacks.
// Register it before the SPA fallback so API failures always return JSON.
func Register(app *fiber.App, db database.Client, sc *sessions.SessionClient, wa *webauthn.WebAuthn) {
	a := &API{db: db, sessions: sc, wa: wa}
	app.Get("/auth/youtube/callback", a.identify, a.handle(a.youtubeCallback, true))
	app.Get("/api/settings/youtube-sync/connect/callback", a.identify, a.handle(a.youtubeCallback, true))
	g := app.Group("/api/v1", a.identify)
	g.Use(limiter.New(limiter.Config{Max: 300, Expiration: time.Minute, KeyGenerator: func(c fiber.Ctx) string {
		if id := viewer(c); id != "" {
			return "user:" + id
		}
		return "ip:" + c.IP()
	}, LimitReached: func(c fiber.Ctx) error {
		c.Set("Retry-After", "60")
		return errorJSON(c, 429, "rate_limited", "Please wait before trying again", nil)
	}}))
	g.Get("/session", a.handle(a.session, false))
	g.Get("/legal/:document", a.handle(a.legal, false))
	g.Post("/logout", a.handle(a.logout, false))
	g.Post("/auth/login/begin", a.handle(a.loginBegin, false))
	g.Post("/auth/login/finish", a.handle(a.loginFinish, false))
	g.Post("/auth/register/begin", a.handle(a.registerBegin, false))
	g.Post("/auth/register/finish", a.handle(a.registerFinish, false))
	g.Post("/passkeys/add/begin", a.handle(a.passkeyBegin, true))
	g.Post("/passkeys/add/finish", a.handle(a.passkeyFinish, true))
	g.Get("/feed", a.handle(a.feed, true))
	g.Get("/history", a.handle(a.history, true))
	g.Get("/watch-later", a.handle(a.watchLater, true))
	g.Put("/watch-later/:id", a.handle(a.setWatchLater, true))
	g.Get("/media/:id", a.handle(a.media, false))
	g.Get("/tv/status", a.handle(a.tvPlayerStatus, true))
	g.Post("/media/:id/tv", a.handle(a.sendToTV, true))
	g.Get("/media/:id/segments", a.handle(a.podcastSegments, true))
	g.Post("/media/:id/segments", a.handle(a.startPodcastSegments, true))
	g.Patch("/media/:id/state", a.handle(a.patchMedia, true))
	g.Get("/subscriptions", a.handle(a.subscriptions, true))
	g.Post("/subscriptions", a.handle(a.subscribe, true))
	g.Delete("/subscriptions/:id", a.handle(a.unsubscribe, true))
	g.Patch("/subscriptions/:id", a.handle(a.subscriptionFilter, true))
	g.Get("/search", a.handle(a.search, true))
	g.Post("/podcasts/open", a.handle(a.openPodcast, true))
	g.Get("/channels/:id", a.handle(a.channel, false))
	g.Post("/channels/:id/refresh", a.handle(a.refreshChannel, true))
	g.Get("/playlists", a.handle(a.playlists, true))
	g.Post("/playlists", a.handle(a.createPlaylist, true))
	g.Post("/playlists/import", a.handle(a.importPlaylist, true))
	g.Get("/playlists/:id", a.handle(a.playlist, true))
	g.Patch("/playlists/:id", a.handle(a.updatePlaylist, true))
	g.Delete("/playlists/:id", a.handle(a.deletePlaylist, true))
	g.Post("/playlists/:id/sync", a.handle(a.syncPlaylist, true))
	g.Post("/playlists/:id/items", a.handle(a.addPlaylistItem, true))
	g.Put("/playlists/:id/items/:mediaId", a.handle(a.setPlaylistItem, true))
	g.Post("/playlists/:id/items/:mediaId/move", a.handle(a.movePlaylistItem, true))
	g.Get("/settings", a.handle(a.settings, true))
	g.Patch("/settings", a.handle(a.patchSettings, true))
	g.Post("/settings/youtube/connect", a.handle(a.connectYouTube, true))
	g.Post("/settings/youtube/disconnect", a.handle(a.disconnectYouTube, true))
	g.Put("/settings/youtube", a.handle(a.enableYouTube, true))
	g.Post("/settings/tv/connect", a.handle(a.connectTV, true))
	g.Delete("/settings/tv", a.handle(a.disconnectTV, true))
	g.Put("/settings/tv", a.handle(a.enableTV, true))
	g.Delete("/passkeys/:id", a.handle(a.deletePasskey, true))
	g.Use(func(c fiber.Ctx) error {
		return errorJSON(c, 404, "not_found", "This API endpoint does not exist", nil)
	})
}

func viewer(c fiber.Ctx) string { id, _ := c.Locals("webapi_user").(string); return id }
func (a *API) identify(c fiber.Ctx) error {
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Method() != "GET" && c.Method() != "HEAD" && c.Method() != "OPTIONS" {
		if c.Get("Sec-Fetch-Site") == "cross-site" {
			return errorJSON(c, 403, "origin_forbidden", "Use this application's origin", nil)
		}
		if origin := c.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != c.Get("Host") {
				return errorJSON(c, 403, "origin_forbidden", "Use this application's origin", nil)
			}
		}
	}
	s, ok := c.Locals("session").(sessions.Session)
	if !ok && a.sessions != nil && c.Cookies("session_id") != "" {
		s, _ = a.sessions.GetReadOnly(c.Context(), c.Cookies("session_id"))
	}
	if s.Valid() {
		c.Locals("webapi_session", s)
	}
	if id, valid := s.UserID(); valid && id != "" {
		c.Locals("webapi_user", id)
		c.Locals("webapi_session", s)
	}
	return c.Next()
}

func body(c fiber.Ctx, out any) error {
	if len(c.Body()) > 32768 {
		return fail(413, "body_too_large", "The request is too large")
	}
	typeName, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || typeName != "application/json" {
		return fail(415, "json_required", "Send an application/json request")
	}
	d := json.NewDecoder(bytes.NewReader(c.Body()))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return fail(400, "invalid_json", "The JSON request is invalid")
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return fail(400, "invalid_json", "Send exactly one JSON object")
	}
	return nil
}

func pagination(c fiber.Ctx) (int, int, error) {
	limit, err := strconv.Atoi(c.Query("limit", "24"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, 0, invalid("limit", "Use a number from 1 to 100")
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil || offset < 0 || offset > 1000000 {
		return 0, 0, invalid("offset", "Use a nonnegative page offset")
	}
	return limit, offset, nil
}
