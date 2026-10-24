package webapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/gofiber/fiber/v3"
)

func installMediaRefreshAPI(f *fixture, fetch func(context.Context, string) (*youtube.VideoDetails, error)) *API {
	a := &API{db: f.db, fetchVideoMetadata: fetch}
	f.app = fiber.New()
	f.app.Get("/api/v1/media/:id", a.handle(a.media, false))
	return a
}

func cacheRefreshVideo(t *testing.T, f *fixture, id, kind string, age time.Duration, duration int64) *models.Video {
	t.Helper()
	v := &models.Video{
		ID: id, ChannelID: "show", Type: kind, Title: "Cached title", Description: "Cached description", Duration: duration,
		CreatedAt: time.Now().Add(-48 * time.Hour), UpdatedAt: time.Now().Add(-age), PublishedAt: time.Now().Add(-24 * time.Hour),
	}
	if kind == "podcast_episode" {
		v.MediaURL = null.StringFrom("https://example.test/audio.mp3")
	}
	if err := f.db.UpsertVideos(boil.SkipTimestamps(context.Background()), v); err != nil {
		t.Fatal(err)
	}
	return v
}

func archivedStreamDetails(id string) *youtube.VideoDetails {
	return &youtube.VideoDetails{
		Video:     youtube.Video{ID: id, Type: youtube.VideoTypeStreamRecording, Title: "Archived stream", Duration: 9915},
		ChannelID: "provider-channel", Duration: 9915,
	}
}

func TestMediaRefreshesStaleStreamBeforeResponding(t *testing.T) {
	f := newFixture(t)
	cached := cacheRefreshVideo(t, f, "yIcNy60mcAg", "upcoming_stream", 2*time.Hour, 0)
	calls := 0
	installMediaRefreshAPI(f, func(ctx context.Context, id string) (*youtube.VideoDetails, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
			t.Fatalf("metadata fetch must have a bounded deadline: %v, %v", deadline, ok)
		}
		return archivedStreamDetails(id), nil
	})

	res, value := f.request(t, "GET", "/media/yIcNy60mcAg", "", "", "")
	item := value.(map[string]any)["item"].(map[string]any)
	if res.StatusCode != 200 || item["type"] != "stream_recording" || item["durationSeconds"] != float64(9915) {
		t.Fatalf("first response still used stale upcoming metadata: %d %#v", res.StatusCode, value)
	}
	stored, err := f.db.GetVideoByID(context.Background(), "yIcNy60mcAg")
	if err != nil || stored.Type != "stream_recording" || stored.Duration != 9915 || stored.ChannelID != "show" {
		t.Fatalf("subsequent playback must see the refreshed recording: %+v, %v", stored, err)
	}
	if !stored.PublishedAt.Equal(cached.PublishedAt) {
		t.Fatalf("refresh without a publication date lost the cached date: %+v", stored)
	}
	f.request(t, "GET", "/media/yIcNy60mcAg", "", "", "")
	if calls != 1 {
		t.Fatalf("fresh metadata fetched again: %d calls", calls)
	}
}

func TestMediaMetadataRefreshFreshnessAndPodcasts(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		age        time.Duration
		wantCalls  int
	}{
		{"fresh upcoming", "upcoming_stream", 30 * time.Minute, 0},
		{"fresh live", "live_stream", 30 * time.Minute, 0},
		{"fresh recording", "stream_recording", 30 * time.Minute, 0},
		{"fresh video", "video", 5 * time.Hour, 0},
		{"stale recording", "stream_recording", 2 * time.Hour, 1},
		{"stale video", "video", 7 * time.Hour, 1},
		{"old podcast", "podcast_episode", 48 * time.Hour, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			v := cacheRefreshVideo(t, f, "video000001", tc.kind, tc.age, 120)
			calls := 0
			a := installMediaRefreshAPI(f, func(ctx context.Context, id string) (*youtube.VideoDetails, error) {
				calls++
				return archivedStreamDetails(id), nil
			})
			a.refreshMediaMetadata(context.Background(), v)
			if calls != tc.wantCalls {
				t.Fatalf("got %d provider calls, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestMediaMetadataRefreshPreservesCachedData(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    string
		details *youtube.VideoDetails
		err     error
	}{
		{name: "provider error", kind: "stream_recording", err: errors.New("provider unavailable")},
		{name: "deadline", kind: "stream_recording", err: context.DeadlineExceeded},
		{name: "missing metadata", kind: "stream_recording"},
		{name: "wrong video", kind: "stream_recording", details: archivedStreamDetails("another0000")},
		{name: "degraded metadata", kind: "stream_recording", details: &youtube.VideoDetails{Video: youtube.Video{ID: "video000001", Type: youtube.VideoTypeFailed}}},
		{name: "short classification", kind: "short", details: &youtube.VideoDetails{Video: youtube.Video{ID: "video000001", Type: youtube.VideoTypeVideo}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			cached := cacheRefreshVideo(t, f, "video000001", tc.kind, 7*time.Hour, 120)
			installMediaRefreshAPI(f, func(context.Context, string) (*youtube.VideoDetails, error) {
				return tc.details, tc.err
			})
			res, value := f.request(t, "GET", "/media/video000001", "", "", "")
			item := value.(map[string]any)["item"].(map[string]any)
			if res.StatusCode != 200 || item["type"] != tc.kind || item["durationSeconds"] != float64(120) || item["title"] != cached.Title {
				t.Fatalf("cached metadata was lost: %d %#v", res.StatusCode, value)
			}
			if tc.err != nil || tc.details == nil || tc.details.ID != cached.ID || tc.details.Type == youtube.VideoTypeFailed {
				stored, err := f.db.GetVideoByID(context.Background(), cached.ID)
				if err != nil || stored.Description != cached.Description || !stored.PublishedAt.Equal(cached.PublishedAt) || !stored.UpdatedAt.Equal(cached.UpdatedAt) {
					t.Fatalf("failed refresh changed cached metadata: %+v, %v", stored, err)
				}
			}
		})
	}
}

func TestMediaMetadataRefreshAcceptsPrivateVideo(t *testing.T) {
	f := newFixture(t)
	cached := cacheRefreshVideo(t, f, "video000001", "video", 7*time.Hour, 120)
	a := installMediaRefreshAPI(f, func(context.Context, string) (*youtube.VideoDetails, error) {
		return &youtube.VideoDetails{Video: youtube.Video{ID: cached.ID, Type: youtube.VideoTypePrivate}}, nil
	})
	a.refreshMediaMetadata(context.Background(), cached)
	stored, err := f.db.GetVideoByID(context.Background(), cached.ID)
	if err != nil || stored.Type != "private" || !stored.Private || stored.Description != "" || !stored.PublishedAt.Equal(cached.PublishedAt) {
		t.Fatalf("legitimate private metadata was not accepted: %+v, %v", stored, err)
	}
}

func TestMediaMetadataRefreshCoalescesAndBoundsProviderWork(t *testing.T) {
	f := newFixture(t)
	videos := make([]*models.Video, 5)
	for i := range videos {
		videos[i] = cacheRefreshVideo(t, f, fmt.Sprintf("video%06d", i), "upcoming_stream", 2*time.Hour, 0)
	}
	started := make(chan string, 16)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int64
	a := installMediaRefreshAPI(f, func(ctx context.Context, id string) (*youtube.VideoDetails, error) {
		calls.Add(1)
		started <- id
		select {
		case <-release:
			return archivedStreamDetails(id), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	var wg sync.WaitGroup
	start := func(v *models.Video) {
		wg.Go(func() { a.refreshMediaMetadata(context.Background(), v) })
	}
	for _, v := range videos[:4] {
		start(v)
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("provider refresh did not start")
		}
	}
	for range 8 {
		start(videos[0])
	}
	coalescedDone := make(chan struct{})
	wg.Go(func() {
		a.refreshMediaMetadata(context.Background(), videos[0])
		close(coalescedDone)
	})
	select {
	case <-coalescedDone:
		t.Fatal("concurrent viewers did not wait for the shared video refresh")
	case <-time.After(30 * time.Millisecond):
	}
	overflowDone := make(chan struct{})
	go func() {
		a.refreshMediaMetadata(context.Background(), videos[4])
		close(overflowDone)
	}()
	select {
	case <-overflowDone:
	case <-time.After(time.Second):
		t.Fatal("refresh beyond the concurrency limit blocked the request")
	}
	unblock()
	wg.Wait()
	if calls.Load() != 4 {
		t.Fatalf("duplicate or excess provider work: %d calls", calls.Load())
	}
	for _, v := range videos[:4] {
		stored, err := f.db.GetVideoByID(context.Background(), v.ID)
		if err != nil || stored.Type != "stream_recording" || stored.Duration != 9915 {
			t.Fatalf("coalesced refresh did not persist metadata: %+v, %v", stored, err)
		}
	}
}
