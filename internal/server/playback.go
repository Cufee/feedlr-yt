package server

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/playback"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/rs/zerolog/log"
)

type playbackBackend interface {
	Resolve(context.Context, string, string) playback.Result
	ResolveAudio(context.Context, string, string) playback.Result
	Manifest(string, string) ([]byte, error)
	OpenMedia(context.Context, string, string, string, string, string) (*http.Response, error)
}

type playbackDatabase interface {
	GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error)
	GetUserViews(context.Context, string, ...string) ([]*models.View, error)
}

type playbackRoutes struct {
	service playbackBackend
	db      playbackDatabase
}

func newPlaybackService() *playback.Service {
	cfg := playback.Config{CompanionURL: os.Getenv("COMPANION_URL"), CompanionKey: os.Getenv("COMPANION_SECRET"), ForceIframe: strings.EqualFold(os.Getenv("NATIVE_PLAYBACK_ENABLED"), "false"), Observe: metrics.ObservePlayback}
	service, err := playback.New(cfg)
	if err != nil {
		// Invalid optional playback configuration must not prevent iframe playback.
		log.Warn().Msg("Native playback configuration invalid; using YouTube player")
		service, _ = playback.New(playback.Config{ForceIframe: true, Observe: metrics.ObservePlayback})
	}
	return service
}

func registerPlaybackRoutes(app *fiber.App, service playbackBackend, db playbackDatabase, sc *sessions.SessionClient) {
	routes := playbackRoutes{service, db}
	authenticate := playbackAuthentication(sc)
	requests := limiter.New(limiter.Config{Max: 30, Expiration: time.Minute, KeyGenerator: func(c *fiber.Ctx) string { return c.Locals("playback_user").(string) }, LimitReached: func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusTooManyRequests) }})
	app.Post("/api/videos/:id/playback", authenticate, playbackSameOrigin, requests, routes.resolve)
	app.Post("/api/videos/:id/playback/events", authenticate, playbackSameOrigin, requests, routes.event)
	// These routes intentionally precede and bypass the generic API limiter and
	// auth refresh. The service holds concurrency slots for the stream lifetime.
	app.Get("/api/playback/:session/manifest.mpd", authenticate, routes.manifest)
	app.Get("/api/playback/:session/media/:resource", authenticate, routes.media)
	app.Head("/api/playback/:session/media/:resource", authenticate, routes.media)
}

func playbackAuthentication(sc *sessions.SessionClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		session, ok := c.Locals("session").(sessions.Session)
		if !ok {
			if sc == nil {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var err error
			session, err = sc.GetReadOnly(ctx, c.Cookies("session_id"))
			if err != nil {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
		}
		user, valid := session.UserID()
		if !valid || user == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
		c.Locals("playback_user", user)
		c.Set("Cache-Control", "private, no-store")
		return c.Next()
	}
}

func playbackSameOrigin(c *fiber.Ctx) error {
	if c.Get("Sec-Fetch-Site") == "cross-site" {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if origin := c.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != c.Get("Host") || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return c.SendStatus(fiber.StatusForbidden)
		}
	}
	return c.Next()
}

var playbackVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func (r playbackRoutes) resolve(c *fiber.Ctx) error {
	// Resolution coalesces work in a goroutine that can outlive this Fiber ctx.
	id := strings.Clone(c.Params("id"))
	if !playbackVideoID.MatchString(id) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	var request struct {
		Mode      string `json:"mode"`
		AudioOnly bool   `json:"audioOnly"`
	}
	if len(c.Body()) > 1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	if len(c.Body()) != 0 && c.BodyParser(&request) != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if request.Mode != "" && request.Mode != "native" && request.Mode != "iframe" {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	user := c.Locals("playback_user").(string)
	result := playback.Result{Mode: "iframe", Reason: "manual"}
	progress := int64(0)
	views, err := r.db.GetUserViews(ctx, user, id)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "progress_unavailable"})
	}
	for _, view := range views {
		if view.VideoID == id {
			progress = max(0, view.Progress)
			break
		}
	}
	video, err := r.db.GetVideoByID(ctx, id)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "video_unavailable"})
	}
	if video.Duration > 0 && progress >= video.Duration {
		progress = 0
	}
	if request.Mode != "iframe" {
		switch youtube.VideoType(video.Type) {
		case youtube.VideoTypeLiveStream, youtube.VideoTypeUpcomingStream:
			result.Reason = "live_or_upcoming"
		case youtube.VideoTypePodcastEpisode:
			return c.SendStatus(fiber.StatusBadRequest)
		default:
			if request.AudioOnly {
				result = r.service.ResolveAudio(ctx, user, id)
			} else {
				result = r.service.Resolve(ctx, user, id)
			}
		}
	}
	metrics.ObservePlayback("selection", result.Reason)
	return c.JSON(fiber.Map{"mode": result.Mode, "reason": result.Reason, "progress": progress, "manifestUrl": result.ManifestURL, "expiresAt": result.ExpiresAt, "audioOnly": result.AudioOnly, "qualities": result.Qualities})
}

func (r playbackRoutes) manifest(c *fiber.Ctx) error {
	manifest, err := r.service.Manifest(c.Locals("playback_user").(string), c.Params("session"))
	if err != nil {
		return playbackError(c, err)
	}
	c.Set("Content-Type", "application/dash+xml")
	return c.Send(manifest)
}

func (r playbackRoutes) media(c *fiber.Ctx) error {
	// fasthttp's request context is reused after the handler and does not expose
	// client disconnect cancellation. The body owns this independent deadline;
	// fasthttp closes it when streaming completes or a downstream write fails.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	response, err := r.service.OpenMedia(ctx, c.Locals("playback_user").(string), c.Params("session"), c.Params("resource"), c.Method(), c.Get("Range"))
	if err != nil {
		cancel()
		switch {
		case errors.Is(err, playback.ErrExpired):
			metrics.ObservePlayback("media", "expired_session")
		case errors.Is(err, playback.ErrNotFound), errors.Is(err, playback.ErrForbidden):
			metrics.ObservePlayback("media", "missing_session")
		case errors.Is(err, playback.ErrBusy):
			metrics.ObservePlayback("media", "busy")
		case errors.Is(err, playback.ErrInvalidRange):
			metrics.ObservePlayback("media", "invalid_range")
		}
		return playbackError(c, err)
	}
	body := &playbackStream{ReadCloser: response.Body, cancel: cancel}
	c.Status(response.StatusCode)
	for _, name := range []string{"Content-Type", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if value := response.Header.Get(name); value != "" {
			c.Set(name, value)
		}
	}
	c.Set("X-Content-Type-Options", "nosniff")
	if response.ContentLength >= 0 {
		c.Response().Header.SetContentLength(int(response.ContentLength))
	}
	if c.Method() == http.MethodHead {
		_ = body.Close()
		return nil
	}
	c.Response().ImmediateHeaderFlush = true
	// SendStream takes ownership; never defer Close here or use c after return.
	return c.SendStream(body, int(response.ContentLength))
}

type playbackStream struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (s *playbackStream) Close() error {
	var err error
	s.once.Do(func() { s.cancel(); err = s.ReadCloser.Close() })
	return err
}

func playbackError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, playback.ErrNotFound), errors.Is(err, playback.ErrForbidden):
		return c.SendStatus(fiber.StatusNotFound)
	case errors.Is(err, playback.ErrExpired):
		return c.SendStatus(fiber.StatusGone)
	case errors.Is(err, playback.ErrBusy):
		return c.SendStatus(fiber.StatusTooManyRequests)
	case errors.Is(err, playback.ErrInvalidRange):
		return c.SendStatus(fiber.StatusRequestedRangeNotSatisfiable)
	default:
		return c.SendStatus(fiber.StatusBadGateway)
	}
}

func (r playbackRoutes) event(c *fiber.Ctx) error {
	if len(c.Body()) > 1024 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	var event struct {
		Event   string  `json:"event"`
		Reason  string  `json:"reason"`
		Seconds float64 `json:"seconds"`
	}
	if c.BodyParser(&event) != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	switch event.Event {
	case "startup", "fallback", "media_error", "renewal", "switch":
	default:
		return c.SendStatus(fiber.StatusBadRequest)
	}
	switch event.Reason {
	case "native", "iframe", "manual", "startup_timeout", "playback_stall",
		"expired_media", "unsupported_browser", "unsupported_codec", "native_error",
		"iframe_failed", "scheduled", "resolution_failed", "invalid_resolution",
		"server_fallback", "ready", "live_or_upcoming", "disabled", "unconfigured",
		"invalid_video", "busy", "resolution_timeout", "health_unavailable",
		"expired_urls", "video_unavailable", "invalid_manifest", "internal_error":
	default:
		event.Reason = "other"
	}
	metrics.ObservePlayback(event.Event, event.Reason)
	if event.Event == "startup" && !math.IsNaN(event.Seconds) && !math.IsInf(event.Seconds, 0) && event.Seconds >= 0 && event.Seconds <= 600 {
		metrics.ObservePlaybackStartup(event.Seconds)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
