package logic

import (
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
)

func TestNormalizeSponsorSegments(t *testing.T) {
	segments := []sponsorblock.Segment{
		{Segment: []float64{10, 20}},
		{Segment: []float64{19.5, 30}},
		{Segment: []float64{40, 40.5}}, // too short and should be dropped
		{Segment: []float64{35, 38}},
	}

	normalized := normalizeSponsorSegments(segments)
	if len(normalized) != 2 {
		t.Fatalf("expected 2 normalized segments, got %d", len(normalized))
	}

	if normalized[0].Start != 10 || normalized[0].End != 30 {
		t.Fatalf("unexpected merged segment: %+v", normalized[0])
	}
	if normalized[1].Start != 35 || normalized[1].End != 38 {
		t.Fatalf("unexpected second segment: %+v", normalized[1])
	}
}

func TestShouldWriteProgress(t *testing.T) {
	now := time.Now().UTC()
	if !shouldWriteProgress("1", now, time.Time{}) {
		t.Fatal("expected first write to be allowed")
	}
	if shouldWriteProgress("1", now, now) {
		t.Fatal("expected write interval guard")
	}
	if shouldWriteProgress("1081", now, time.Time{}) {
		t.Fatal("expected ad/unknown state to be ignored")
	}
}

func TestClampPlaybackSecond(t *testing.T) {
	if got := clampPlaybackSecond(42.9, 40, true); got != 40 {
		t.Fatalf("expected clamp to duration, got %d", got)
	}
	if got := clampPlaybackSecond(-5, 0, false); got != 0 {
		t.Fatalf("expected clamp to zero, got %d", got)
	}
}

func TestShouldAttemptResumeSeek(t *testing.T) {
	playbackAtStart := lounge.PlaybackEvent{
		HasCurrentTime: true,
		CurrentTime:    10,
	}
	if !shouldAttemptResumeSeek(true, playbackAtStart, "1", "") {
		t.Fatal("expected resume attempt for new video near start")
	}

	playbackOngoing := lounge.PlaybackEvent{
		HasCurrentTime: true,
		CurrentTime:    300,
	}
	if shouldAttemptResumeSeek(true, playbackOngoing, "1", "") {
		t.Fatal("did not expect resume attempt for new video that is already ongoing")
	}

	if !shouldAttemptResumeSeek(false, playbackOngoing, "1", "2") {
		t.Fatal("expected resume attempt on transition into playing state")
	}

	if shouldAttemptResumeSeek(false, playbackOngoing, "1", "") {
		t.Fatal("did not expect resume attempt on first seen ongoing playback state")
	}
}

func TestShouldApplyResumeSeek(t *testing.T) {
	playbackAtStart := lounge.PlaybackEvent{
		HasCurrentTime: true,
		CurrentTime:    10,
	}
	if !shouldApplyResumeSeek(42, playbackAtStart) {
		t.Fatal("expected seek near start with saved progress")
	}

	playbackOngoing := lounge.PlaybackEvent{
		HasCurrentTime: true,
		CurrentTime:    300,
	}
	if !shouldApplyResumeSeek(320, playbackOngoing) {
		t.Fatal("expected seek when app progress is sufficiently ahead")
	}
	if shouldApplyResumeSeek(305, playbackOngoing) {
		t.Fatal("did not expect seek when app progress is not ahead enough")
	}
	if shouldApplyResumeSeek(0, playbackOngoing) {
		t.Fatal("did not expect seek without saved progress")
	}
	if shouldApplyResumeSeek(320, lounge.PlaybackEvent{}) {
		t.Fatal("did not expect seek without current time")
	}
}

func TestTVSyncRuntimeResumeAppliedFlagResetsOnVideoChange(t *testing.T) {
	runtime := newTVSyncRuntime(false, nil)
	if runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("did not expect resume flag before any video")
	}

	if !runtime.setCurrentVideo("video-a") {
		t.Fatal("expected first video assignment to be treated as new")
	}
	runtime.markResumeAppliedForCurrentVideo()
	if !runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("expected resume flag to be set for active video")
	}

	if runtime.setCurrentVideo("video-a") {
		t.Fatal("did not expect same video to be treated as new")
	}
	if !runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("expected resume flag to remain set for same active video")
	}

	if !runtime.setCurrentVideo("video-b") {
		t.Fatal("expected video change to be treated as new")
	}
	if runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("expected resume flag reset after active video changed")
	}
}

func TestTVSyncRuntimePlaybackSnapshotAndReset(t *testing.T) {
	runtime := newTVSyncRuntime(false, nil)

	videoID, state := runtime.currentPlaybackSnapshot()
	if videoID != "" || state != "" {
		t.Fatalf("expected empty playback snapshot, got video=%q state=%q", videoID, state)
	}

	if !runtime.setCurrentVideo("video-a") {
		t.Fatal("expected first video assignment")
	}
	runtime.setCurrentPlaybackState("1")

	videoID, state = runtime.currentPlaybackSnapshot()
	if videoID != "video-a" || state != "1" {
		t.Fatalf("unexpected playback snapshot, got video=%q state=%q", videoID, state)
	}

	if !runtime.setCurrentVideo("video-b") {
		t.Fatal("expected video change to be treated as new")
	}
	videoID, state = runtime.currentPlaybackSnapshot()
	if videoID != "video-b" || state != "" {
		t.Fatalf("expected playback state reset on video change, got video=%q state=%q", videoID, state)
	}

	runtime.clearCurrentVideo()
	videoID, state = runtime.currentPlaybackSnapshot()
	if videoID != "" || state != "" {
		t.Fatalf("expected cleared playback snapshot, got video=%q state=%q", videoID, state)
	}
}
