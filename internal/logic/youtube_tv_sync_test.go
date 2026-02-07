package logic

import (
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
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

func TestShouldForceSeekToStoredProgress(t *testing.T) {
	if !shouldForceSeekToStoredProgress(true) {
		t.Fatal("expected seek for new video")
	}
	if shouldForceSeekToStoredProgress(false) {
		t.Fatal("did not expect seek for existing video state")
	}
}
