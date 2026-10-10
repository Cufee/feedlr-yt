package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/gofiber/fiber/v2"
)

func TestChapterCacheCoalescesAndExpires(t *testing.T) {
	cache := newChapterCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	var calls atomic.Int64
	cache.load = func(ctx context.Context, id string, duration float64, description string) ([]youtube.Chapter, error) {
		calls.Add(1)
		return []youtube.Chapter{{Start: 0, End: duration, Title: "Intro"}}, nil
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			got := cache.get(context.Background(), "abcdefghijk", 60, "")
			if len(got) != 1 || got[0].End != 60 {
				t.Errorf("bad cached chapters: %+v", got)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("duplicate upstream calls: %d", calls.Load())
	}
	now = now.Add(6*time.Hour + time.Second)
	cache.get(context.Background(), "abcdefghijk", 60, "")
	if calls.Load() != 2 {
		t.Fatal("expired cache did not retry")
	}
}

func TestChapterCacheFailureHasShortTTLAndBoundedSize(t *testing.T) {
	cache := newChapterCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	var calls atomic.Int64
	cache.load = func(context.Context, string, float64, string) ([]youtube.Chapter, error) {
		calls.Add(1)
		return nil, errors.New("upstream unavailable")
	}
	cache.get(context.Background(), "abcdefghijk", 60, "")
	cache.get(context.Background(), "abcdefghijk", 60, "")
	if calls.Load() != 1 {
		t.Fatal("failure was not cached")
	}
	now = now.Add(61 * time.Second)
	cache.get(context.Background(), "abcdefghijk", 60, "")
	if calls.Load() != 2 {
		t.Fatal("failure did not expire promptly")
	}
	for i := range 300 {
		cache.get(context.Background(), fmt.Sprint(i), 60, "")
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) > 256 {
		t.Fatalf("unbounded cache: %d", len(cache.entries))
	}
}

func TestChapterCacheCancellationAndRequestLimit(t *testing.T) {
	cache := newChapterCache()
	started := make(chan struct{}, 10)
	release := make(chan struct{})
	cache.load = func(ctx context.Context, id string, duration float64, description string) ([]youtube.Chapter, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := range 8 {
		cache.get(ctx, fmt.Sprint(i), 60, "")
		<-started
	}
	description := "0:00 Intro\n0:20 Middle\n0:40 End"
	chapters := cache.get(context.Background(), "ninth", 60, description)
	if len(chapters) != 3 {
		t.Fatal("saturated requests did not use description fallback")
	}
	cache.mu.Lock()
	flights := make([]*chapterFlight, 0, len(cache.flights))
	for _, flight := range cache.flights {
		flights = append(flights, flight)
	}
	cache.mu.Unlock()
	close(release)
	for _, flight := range flights {
		<-flight.done
	}
}

type chapterTestDB struct {
	fakePlaybackDB
	video *models.Video
	err   error
}

func (d chapterTestDB) GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error) {
	return d.video, d.err
}

func TestChapterRouteValidatesVideoAndReturnsEmptyArray(t *testing.T) {
	for _, tc := range []struct {
		id    string
		video *models.Video
		err   error
		code  int
	}{
		{"bad", &models.Video{}, nil, 400},
		{"abcdefghijk", &models.Video{Type: "podcast_episode"}, nil, 400},
		{"abcdefghijk", nil, errors.New("database unavailable"), 503},
		{"abcdefghijk", &models.Video{Type: "video", Duration: 60}, nil, 200},
	} {
		cache := newChapterCache()
		cache.load = func(context.Context, string, float64, string) ([]youtube.Chapter, error) { return nil, nil }
		routes := chapterRoutes{db: &chapterTestDB{video: tc.video, err: tc.err}, cache: cache}
		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		app.Get("/api/videos/:id/chapters", routes.chapters)
		res, err := app.Test(httptest.NewRequest("GET", "/api/videos/"+tc.id+"/chapters", nil))
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != tc.code {
			t.Errorf("%s: got %d, want %d", tc.id, res.StatusCode, tc.code)
		}
		if tc.code == 200 {
			var result struct {
				Chapters []youtube.Chapter `json:"chapters"`
			}
			if json.NewDecoder(res.Body).Decode(&result) != nil || result.Chapters == nil {
				t.Fatal("no metadata must produce an empty JSON array")
			}
		}
		res.Body.Close()
	}
}

func TestChapterRouteRequiresAuthenticationIncludingThumbnails(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	registerPlaybackRoutes(app, &fakePlayback{}, &fakePlaybackDB{}, nil)
	for _, method := range []string{"GET", "HEAD"} {
		for _, path := range []string{"/api/videos/abcdefghijk/chapters", "/api/videos/abcdefghijk/chapters/0/thumbnail"} {
			if method == "HEAD" && path == "/api/videos/abcdefghijk/chapters" {
				continue
			}
			res, err := app.Test(httptest.NewRequest(method, path, nil))
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != 401 {
				t.Fatalf("%s %s bypassed authentication: %d", method, path, res.StatusCode)
			}
		}
	}
}

func TestChapterCacheMissingClientUsesStoredDescription(t *testing.T) {
	previous := youtube.DefaultClient
	youtube.DefaultClient = nil
	defer func() { youtube.DefaultClient = previous }()
	chapters := newChapterCache().get(context.Background(), "abcdefghijk", 60, "0:00 Intro\n0:20 Middle\n0:40 End")
	if len(chapters) != 3 || chapters[2].End != 60 {
		t.Fatalf("fallback missing: %+v", chapters)
	}
}
