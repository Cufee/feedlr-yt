package api

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
)

type podcastSegmentsResponse struct {
	Status          string                   `json:"status"`
	Enabled         bool                     `json:"enabled"`
	Phase           string                   `json:"phase,omitempty"`
	Error           string                   `json:"error,omitempty"`
	Source          string                   `json:"source,omitempty"`
	TranscriptReady bool                     `json:"transcript_ready"`
	HasSegments     bool                     `json:"has_segments"`
	DurationMS      int                      `json:"duration_ms,omitempty"`
	PollAfterMS     int                      `json:"poll_after_ms,omitempty"`
	Segments        []podcastSegmentResponse `json:"segments"`
}
type podcastSegmentResponse struct {
	Category  string `json:"category"`
	StartMS   int    `json:"start_ms"`
	EndMS     int    `json:"end_ms"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	StartText string `json:"start_text"`
	EndText   string `json:"end_text"`
	Reason    string `json:"reason"`
	Brand     string `json:"brand,omitempty"`
	Skippable bool   `json:"skippable"`
}

// GET is cache-only. POST is the sole listener-driven kickoff route.
var PodcastSponsorSegments = podcastSponsorSegments(false)
var StartPodcastSponsorSegments = podcastSponsorSegments(true)

func podcastSponsorSegments(start bool) brewed.Partial[*handler.Context] {
	return func(ctx *handler.Context) (templ.Component, error) {
		uid, ok := ctx.UserID()
		if !ok {
			return nil, podcastSegmentsJSON(ctx, http.StatusUnauthorized, nil)
		}
		settings, err := logic.GetUserSettings(ctx.Context(), ctx.Database(), uid)
		if err != nil {
			return nil, err
		}
		if !settings.PodcastSegments.Enabled {
			if start {
				return nil, podcastSegmentsJSON(ctx, http.StatusForbidden, nil)
			}
			return nil, podcastSegmentsJSON(ctx, http.StatusOK, podcastSegmentsResponse{Status: "disabled", Segments: []podcastSegmentResponse{}})
		}
		var status logic.PodcastSegmentStatus
		if start {
			status, err = logic.EnsurePodcastSegmentAnalysis(ctx.Context(), ctx.Database(), ctx.Params("id"))
		} else {
			status, err = logic.GetPodcastSegmentStatus(ctx.Context(), ctx.Database(), ctx.Params("id"))
		}
		if err != nil {
			return nil, podcastSegmentsJSON(ctx, http.StatusBadRequest, nil)
		}
		response := podcastSegmentSnapshot(status, settings.PodcastSegments.SelectedCategories)
		return nil, podcastSegmentsJSON(ctx, http.StatusOK, response)
	}
}

func podcastSegmentSnapshot(status logic.PodcastSegmentStatus, selectedCategories []string) podcastSegmentsResponse {
	response := podcastSegmentsResponse{Enabled: true, Status: status.Status, Phase: status.Phase, Error: status.Error, Source: status.Source, TranscriptReady: status.TranscriptReady, HasSegments: len(status.Segments) > 0, DurationMS: status.DurationMS, Segments: []podcastSegmentResponse{}}
	if status.Status == database.PodcastSegmentPending || status.Status == database.PodcastSegmentRunning {
		response.PollAfterMS = 2000
	}
	// Confirmed snapshots remain useful while the rest of a scan is running
	// or has failed. Repeated publication must not create duplicate rows.
	type identity struct {
		category, brand string
		start, end      int
	}
	seen := make(map[identity]bool)
	for _, segment := range status.Segments {
		if !slices.Contains(selectedCategories, segment.Category) {
			continue
		}
		key := identity{segment.Category, segment.Brand, segment.StartMS, segment.EndMS}
		if seen[key] {
			continue
		}
		seen[key] = true
		response.Segments = append(response.Segments, podcastSegmentResponse{Category: segment.Category, StartMS: segment.StartMS, EndMS: segment.EndMS, StartTime: logic.FormatPodcastSegmentTime(segment.StartMS), EndTime: logic.FormatPodcastSegmentTime(segment.EndMS), StartText: segment.StartText, EndText: segment.EndText, Reason: segment.Reason, Brand: segment.Brand, Skippable: true})
	}
	return response
}

func podcastSegmentsJSON(ctx *handler.Context, status int, body any) error {
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
