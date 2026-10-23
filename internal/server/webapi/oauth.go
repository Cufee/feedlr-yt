package webapi

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

func (a *API) youtubeCallback(c fiber.Ctx) error {
	if logic.DefaultYouTubeSync == nil {
		return fail(503, "youtube_sync_unavailable", "YouTube synchronization is not configured")
	}
	state := c.Cookies("youtube_sync_state")
	c.Cookie(&fiber.Cookie{Name: "youtube_sync_state", Value: "", Path: "/", HTTPOnly: true, SameSite: "Lax", Expires: time.Unix(1, 0), MaxAge: -1})
	expected := state + ":" + viewer(c)
	if state == "" || subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(expected)) != 1 {
		return fail(400, "invalid_oauth_state", "The YouTube connection request expired. Start again.")
	}
	if c.Query("error") != "" {
		return fail(400, "oauth_cancelled", "The YouTube connection was canceled")
	}
	code := c.Query("code")
	if code == "" {
		return fail(400, "oauth_code_missing", "YouTube did not provide an authorization code")
	}
	service := logic.DefaultYouTubeSync
	if err := service.CompleteOAuth(c.Context(), viewer(c), code); err != nil {
		return fail(502, "oauth_failed", "YouTube could not be connected. Please try again.")
	}
	userID := strings.Clone(viewer(c))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := service.RunSyncForUser(ctx, userID); err != nil {
			log.Warn().Err(err).Msg("initial YouTube synchronization failed")
		}
	}()
	return c.Redirect().Status(303).To("/app/settings/youtube")
}
