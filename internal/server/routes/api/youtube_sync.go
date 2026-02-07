package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/cufee/feedlr-yt/internal/logic"
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
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
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
	return ctx.Redirect(redirectURL, http.StatusTemporaryRedirect)
}

var FinishYouTubeSyncConnect brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
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
		return ctx.Err(errors.New("invalid oauth state"))
	}

	code := ctx.Query("code")
	if code == "" {
		return ctx.Err(errors.New("missing oauth code"))
	}

	err = service.CompleteOAuth(ctx.Context(), userID, code)
	if err != nil {
		return ctx.Err(err)
	}

	go func(userID string) {
		runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := service.RunSyncForUser(runCtx, userID); err != nil {
			log.Warn().Err(err).Str("userID", userID).Msg("initial youtube sync failed")
		}
	}(userID)

	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var DisconnectYouTubeSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		return ctx.Err(err)
	}

	err = service.Disconnect(ctx.Context(), userID)
	if err != nil {
		return ctx.Err(err)
	}

	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ToggleYouTubeSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeSyncService()
	if err != nil {
		return ctx.Err(err)
	}

	enabled, err := parseEnabledForm(ctx)
	if err != nil {
		return ctx.Err(err)
	}

	err = service.SetEnabled(ctx.Context(), userID, enabled)
	if err != nil {
		return ctx.Err(err)
	}
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ConnectYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		return ctx.Err(err)
	}

	pairingCode, err := ctx.FormValue("pairing_code")
	if err != nil {
		return ctx.Err(err)
	}
	if pairingCode == "" {
		return ctx.Err(errors.New("missing pairing code"))
	}

	err = service.PairWithCode(ctx.Context(), userID, pairingCode)
	if err != nil {
		return ctx.Err(err)
	}

	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var DisconnectYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		return ctx.Err(err)
	}

	err = service.Disconnect(ctx.Context(), userID)
	if err != nil {
		return ctx.Err(err)
	}

	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}

var ToggleYouTubeTVSync brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return ctx.SendStatus(http.StatusUnauthorized)
	}

	service, err := youtubeTVSyncService()
	if err != nil {
		return ctx.Err(err)
	}

	enabled, err := parseEnabledForm(ctx)
	if err != nil {
		return ctx.Err(err)
	}

	err = service.SetEnabled(ctx.Context(), userID, enabled)
	if err != nil {
		return ctx.Err(err)
	}
	return ctx.Redirect("/app/settings", http.StatusTemporaryRedirect)
}
