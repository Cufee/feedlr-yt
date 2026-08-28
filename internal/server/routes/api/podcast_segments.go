package api

import (
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/tpot/brewed"
)

type podcastSegmentsResponse struct {
	Status      string                   `json:"status"`
	PollAfterMS int                      `json:"poll_after_ms,omitempty"`
	Segments    []podcastSegmentResponse `json:"segments"`
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

var PodcastSponsorSegments brewed.Partial[*handler.Context] = func(ctx *handler.Context) (templ.Component, error) {
	uid, ok := ctx.UserID()
	if !ok {
		return nil, ctx.SendStatus(http.StatusUnauthorized)
	}
	waitMS, _ := strconv.Atoi(ctx.Query("wait_ms"))
	if waitMS < 0 {
		waitMS = 0
	}
	if waitMS > 3000 {
		waitMS = 3000
	}
	deadline := time.Now().Add(time.Duration(waitMS) * time.Millisecond)
	var status logic.PodcastSegmentStatus
	var err error
	for {
		status, err = logic.EnsurePodcastSegmentAnalysis(ctx.Context(), ctx.Database(), ctx.Params("id"))
		if err != nil {
			return nil, ctx.SendStatus(http.StatusBadRequest)
		}
		if status.Status != database.PodcastSegmentPending && status.Status != database.PodcastSegmentRunning || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	response := podcastSegmentsResponse{Status: status.Status, Segments: []podcastSegmentResponse{}}
	if status.Status == database.PodcastSegmentPending || status.Status == database.PodcastSegmentRunning {
		response.PollAfterMS = 2000
	}
	settings, err := logic.GetUserSettings(ctx.Context(), ctx.Database(), uid)
	if err != nil {
		return nil, err
	}
	for _, segment := range status.Segments {
		selected := settings.PodcastSegments.Enabled && slices.Contains(settings.PodcastSegments.SelectedCategories, segment.Category)
		if !selected {
			continue
		}
		response.Segments = append(response.Segments, podcastSegmentResponse{Category: segment.Category, StartMS: segment.StartMS, EndMS: segment.EndMS, StartTime: logic.FormatPodcastSegmentTime(segment.StartMS), EndTime: logic.FormatPodcastSegmentTime(segment.EndMS), StartText: segment.StartText, EndText: segment.EndText, Reason: segment.Reason, Brand: segment.Brand, Skippable: selected})
	}
	return nil, ctx.JSON(response)
}
