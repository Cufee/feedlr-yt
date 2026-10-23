package webapi

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v3"
)

func (a *API) tvPlayerStatus(c fiber.Ctx) error {
	service := logic.DefaultYouTubeTVSync
	if service == nil {
		return c.JSON(logic.TVPlayerStatus{})
	}
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	status, err := service.PlayerStatus(ctx, viewer(c))
	if err != nil {
		return fail(503, "tv_unavailable", "Could not check TV availability")
	}
	return c.JSON(status)
}

func (a *API) sendToTV(c fiber.Ctx) error {
	var request struct {
		Position *float64 `json:"position"`
	}
	if err := body(c, &request); err != nil {
		return err
	}
	if request.Position == nil || *request.Position < 0 {
		return invalid("position", "Choose a valid playback position")
	}
	service := logic.DefaultYouTubeTVSync
	if service == nil {
		return fail(409, "tv_offline", "Your TV is offline")
	}
	if err := service.SendVideo(c.Context(), viewer(c), c.Params("id"), *request.Position); err != nil {
		metrics.IncUserAction("send_video_to_tv", "error")
		switch {
		case errors.Is(err, logic.ErrTVOffline):
			return fail(409, "tv_offline", "Your TV is offline")
		case errors.Is(err, logic.ErrInvalidTVVideo):
			return invalid("mediaId", "Choose a playable YouTube video and position")
		default:
			return fail(502, "tv_send_failed", "Could not send the video to your TV. Please try again.")
		}
	}
	metrics.IncUserAction("send_video_to_tv", "success")
	return c.SendStatus(fiber.StatusNoContent)
}

type segmentAnalysisReply struct {
	types.PodcastSegmentAnalysisProps
	PollAfterMS int `json:"poll_after_ms,omitempty"`
}

func (a *API) podcastSegments(c fiber.Ctx) error      { return a.segmentAnalysis(c, false) }
func (a *API) startPodcastSegments(c fiber.Ctx) error { return a.segmentAnalysis(c, true) }

func (a *API) segmentAnalysis(c fiber.Ctx, start bool) error {
	settings, err := logic.GetUserSettings(c.Context(), a.db, viewer(c))
	if err != nil {
		return err
	}
	response := segmentAnalysisReply{PodcastSegmentAnalysisProps: types.PodcastSegmentAnalysisProps{
		Enabled: settings.PodcastSegments.Enabled, Status: "disabled", Segments: []types.PodcastSegmentProps{},
	}}
	if !settings.PodcastSegments.Enabled {
		if start {
			return fail(403, "skipping_disabled", "Enable smart skipping in your podcast settings first")
		}
		return c.JSON(response)
	}
	var status logic.PodcastSegmentStatus
	if start {
		status, err = logic.EnsurePodcastSegmentAnalysis(c.Context(), a.db, c.Params("id"))
	} else {
		status, err = logic.GetPodcastSegmentStatus(c.Context(), a.db, c.Params("id"))
	}
	if err != nil {
		if database.IsErrNotFound(err) {
			return fail(404, "not_found", "This episode is unavailable")
		}
		return fail(400, "analysis_unavailable", "Could not load segment analysis for this episode")
	}
	response.Status, response.Phase, response.Error = status.Status, status.Phase, status.Error
	response.Source, response.TranscriptReady, response.DurationMS = status.Source, status.TranscriptReady, status.DurationMS
	if status.Status == database.PodcastSegmentPending || status.Status == database.PodcastSegmentRunning {
		response.PollAfterMS = 2000
	}
	type identity struct {
		category, brand string
		start, end      int
	}
	seen := map[identity]bool{}
	for _, segment := range status.Segments {
		if !slices.Contains(settings.PodcastSegments.SelectedCategories, segment.Category) {
			continue
		}
		key := identity{segment.Category, segment.Brand, segment.StartMS, segment.EndMS}
		if seen[key] {
			continue
		}
		seen[key] = true
		response.Segments = append(response.Segments, types.PodcastSegmentProps{
			Category: segment.Category, StartMS: segment.StartMS, EndMS: segment.EndMS,
			StartText: segment.StartText, EndText: segment.EndText, Brand: segment.Brand,
			Reason: segment.Reason, Skippable: true,
		})
	}
	response.HasSegments = len(response.Segments) > 0
	return c.JSON(response)
}
