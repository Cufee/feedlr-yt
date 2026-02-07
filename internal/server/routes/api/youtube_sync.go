package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const youtubeSyncStateCookie = "youtube_sync_state"

func youtubeSyncService() (*logic.YouTubeSyncService, error) {
	if logic.DefaultYouTubeSync == nil {
		return nil, errors.New("youtube sync is disabled")
	}
	return logic.DefaultYouTubeSync, nil
}

func youtubeTVSyncService() (*logic.YouTubeTVSyncService, error) {
	if logic.DefaultYouTubeTVSync == nil {
		return nil, errors.New("youtube tv sync is disabled")
	}
	return logic.DefaultYouTubeTVSync, nil
}

func parseEnabledForm(ctx *handler.Context) (bool, error) {
	rawEnabled, err := ctx.FormValue("enabled")
	if err != nil {
		return false, err
	}
	enabled, err := strconv.ParseBool(rawEnabled)
	if err != nil {
		return false, errors.New("invalid enabled value")
	}
	return enabled, nil
}

var BeginYouTubeSyncConnect brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_sync_begin_connect", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_sync_begin_connect", "service_unavailable")
		return ctx.Err(err)
	}

	state := uuid.NewString()
	ctx.Cookie(&fiber.Cookie{
		Name:     youtubeSyncStateCookie,
		Value:    state,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		Expires:  time.Now().Add(10 * time.Minute),
	})

	redirectURL := service.OAuthAuthURL(state + ":" + userID)
	metrics.IncUserAction("youtube_sync_begin_connect", "success")
	return ctx.Redirect(redirectURL, http.StatusTemporaryRedirect)
}

var FinishYouTubeSyncConnect brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_sync_finish_connect", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_sync_finish_connect", "service_unavailable")
		return ctx.Err(err)
	}

	queryState := ctx.Query("state")
	cookieState := ctx.Cookies(youtubeSyncStateCookie)
	ctx.Cookie(&fiber.Cookie{
		Name:     youtubeSyncStateCookie,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Expires:  time.Now().Add(-time.Hour),
	})

	if queryState == "" || cookieState == "" || queryState != cookieState+":"+userID {
		metrics.IncUserAction("youtube_sync_finish_connect", "invalid_state")
		return ctx.Err(errors.New("invalid oauth state"))
	}

	code := ctx.Query("code")
	if code == "" {
		metrics.IncUserAction("youtube_sync_finish_connect", "missing_code")
		return ctx.Err(errors.New("missing oauth code"))
	}

	err = service.CompleteOAuth(ctx.Context(), userID, code)
	if err != nil {
		metrics.IncUserAction("youtube_sync_finish_connect", "error")
		return ctx.Err(err)
	}

	go func(userID string) {
		runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := service.RunSyncForUser(runCtx, userID); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("initial youtube sync failed")
		}
	}(userID)

	metrics.IncUserAction("youtube_sync_finish_connect", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var DisconnectYouTubeSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_sync_disconnect", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_sync_disconnect", "service_unavailable")
		return ctx.Err(err)
	}

	err = service.Disconnect(ctx.Context(), userID)
	if err != nil {
		metrics.IncUserAction("youtube_sync_disconnect", "error")
		return ctx.Err(err)
	}

	metrics.IncUserAction("youtube_sync_disconnect", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ToggleYouTubeSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_sync_toggle", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_sync_toggle", "service_unavailable")
		return ctx.Err(err)
	}

	enabled, err := parseEnabledForm(ctx)
	if err != nil {
		metrics.IncUserAction("youtube_sync_toggle", "invalid_request")
		return ctx.Err(err)
	}

	err = service.SetEnabled(ctx.Context(), userID, enabled)
	if err != nil {
		metrics.IncUserAction("youtube_sync_toggle", "error")
		return ctx.Err(err)
	}
	metrics.IncUserAction("youtube_sync_toggle", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ConnectYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_tv_sync_connect", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_connect", "service_unavailable")
		return ctx.Err(err)
	}

	pairingCode, err := ctx.FormValue("pairing_code")
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_connect", "invalid_request")
		return ctx.Err(err)
	}
	if pairingCode == "" {
		metrics.IncUserAction("youtube_tv_sync_connect", "invalid_request")
		return ctx.Err(errors.New("missing pairing code"))
	}

	err = service.PairWithCode(ctx.Context(), userID, pairingCode)
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_connect", "error")
		return ctx.Err(err)
	}

	metrics.IncUserAction("youtube_tv_sync_connect", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var DisconnectYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_tv_sync_disconnect", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_disconnect", "service_unavailable")
		return ctx.Err(err)
	}

	err = service.Disconnect(ctx.Context(), userID)
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_disconnect", "error")
		return ctx.Err(err)
	}

	metrics.IncUserAction("youtube_tv_sync_disconnect", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ToggleYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		metrics.IncUserAction("youtube_tv_sync_toggle", "unauthorized")
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_toggle", "service_unavailable")
		return ctx.Err(err)
	}

	enabled, err := parseEnabledForm(ctx)
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_toggle", "invalid_request")
		return ctx.Err(err)
	}

	err = service.SetEnabled(ctx.Context(), userID, enabled)
	if err != nil {
		metrics.IncUserAction("youtube_tv_sync_toggle", "error")
		return ctx.Err(err)
	}
	metrics.IncUserAction("youtube_tv_sync_toggle", "success")
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}
