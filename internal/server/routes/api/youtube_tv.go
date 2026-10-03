package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
)

var TVPlayerStatus brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return tvResponse(ctx, http.StatusUnauthorized, nil)
	}
	service := logic.DefaultYouTubeTVSync
	if service == nil {
		return tvResponse(ctx, http.StatusOK, logic.TVPlayerStatus{})
	}
	requestCtx, cancel := context.WithTimeout(ctx.Context(), 2*time.Second)
	defer cancel()
	status, err := service.PlayerStatus(requestCtx, userID)
	if err != nil {
		return tvResponse(ctx, http.StatusServiceUnavailable, map[string]string{"error": "Could not check TV availability"})
	}
	return tvResponse(ctx, http.StatusOK, status)
}

var SendVideoToTV brewed.Endpoint[*handler.Context] = func(ctx *handler.Context) error {
	userID, ok := ctx.UserID()
	if !ok {
		return tvResponse(ctx, http.StatusUnauthorized, nil)
	}
	var request struct {
		Position *float64 `json:"position"`
	}
	if ctx.BodyParser(&request) != nil || request.Position == nil {
		return tvResponse(ctx, http.StatusBadRequest, map[string]string{"error": "Invalid video position"})
	}
	service := logic.DefaultYouTubeTVSync
	if service == nil {
		return tvResponse(ctx, http.StatusConflict, map[string]string{"error": "TV is offline"})
	}
	err := service.SendVideo(ctx.Context(), userID, ctx.Params("id"), *request.Position)
	if err != nil {
		metrics.IncUserAction("send_video_to_tv", "error")
		switch {
		case errors.Is(err, logic.ErrTVOffline):
			return tvResponse(ctx, http.StatusConflict, map[string]string{"error": "TV is offline"})
		case errors.Is(err, logic.ErrInvalidTVVideo):
			return tvResponse(ctx, http.StatusBadRequest, map[string]string{"error": "Invalid video or position"})
		default:
			return tvResponse(ctx, http.StatusBadGateway, map[string]string{"error": "Could not send video to TV. Please try again."})
		}
	}
	metrics.IncUserAction("send_video_to_tv", "success")
	return tvResponse(ctx, http.StatusNoContent, nil)
}

// These routes pass through tpot's net/http adapter. Write through its response
// writer so Fiber does not replace the status or JSON content type afterward.
func tvResponse(ctx *handler.Context, status int, body any) error {
	w := ctx.Writer()
	w.Header().Set("Cache-Control", "private, no-store")
	if body != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	if body == nil {
		return nil
	}
	return json.NewEncoder(w).Encode(body)
}
