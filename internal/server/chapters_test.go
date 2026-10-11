package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

func TestChapterCacheCoalescesAndExpires(t *testing.T) {
	cache := newChapterCache()
	now := time.Now()
	cache.now = func() time.Time { return now }
	var calls atomic.Int64
	cache.load = func(ctx context.Context, id string, duration float64, description string) (youtube.VideoTimeline, error) {
		calls.Add(1)
		return youtube.VideoTimeline{Chapters: []youtube.Chapter{{Start: 0, End: duration, Title: "Intro"}}}, nil
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			got := cache.get(context.Background(), "abcdefghijk", 60, "")
			if len(got.Chapters) != 1 || got.Chapters[0].End != 60 {
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
	cache.load = func(context.Context, string, float64, string) (youtube.VideoTimeline, error) {
		calls.Add(1)
		return youtube.VideoTimeline{}, errors.New("upstream unavailable")
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
	cache.load = func(ctx context.Context, id string, duration float64, description string) (youtube.VideoTimeline, error) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return youtube.VideoTimeline{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := range 8 {
		cache.get(ctx, fmt.Sprint(i), 60, "")
		<-started
	}
	description := "0:00 Intro\n0:20 Middle\n0:40 End"
	chapters := cache.get(context.Background(), "ninth", 60, description)
	if len(chapters.Chapters) != 3 {
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
		cache.load = func(context.Context, string, float64, string) (youtube.VideoTimeline, error) {
			return youtube.VideoTimeline{}, nil
		}
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
		for _, path := range []string{"/api/videos/abcdefghijk/chapters", "/api/videos/abcdefghijk/chapters/0/thumbnail", "/api/videos/abcdefghijk/storyboards/0/thumbnail"} {
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
	if len(chapters.Chapters) != 3 || chapters.Chapters[2].End != 60 {
		t.Fatalf("fallback missing: %+v", chapters)
	}
}

func TestChapterCachePreservesStoryboardsWithoutChapters(t *testing.T) {
	for _, description := range []string{"", "0:00 Intro\n0:20 Middle\n0:40 End"} {
		t.Run(description, func(t *testing.T) {
			cache := newChapterCache()
			now := time.Now()
			cache.now = func() time.Time { return now }
			storyboard := &youtube.Storyboard{Duration: 60, Interval: 10, Count: 6, Width: 160, Height: 90, Columns: 3, Rows: 2, Sheets: []string{"https://i.ytimg.com/sb/abcdefghijk/storyboard3_L0/M0.jpg"}}
			var calls int
			cache.load = func(context.Context, string, float64, string) (youtube.VideoTimeline, error) {
				calls++
				return youtube.VideoTimeline{Storyboard: storyboard}, nil
			}
			wantChapters := 0
			if description != "" {
				wantChapters = 3
			}
			for range 2 {
				timeline := cache.get(context.Background(), "abcdefghijk", 60, description)
				if timeline.Storyboard != storyboard || len(timeline.Chapters) != wantChapters {
					t.Fatalf("storyboard or description fallback lost: %+v", timeline)
				}
			}
			// Storyboard-only metadata is useful and should receive the success TTL.
			now = now.Add(2 * time.Hour)
			if cached := cache.peek("abcdefghijk"); cached.Storyboard != storyboard {
				t.Fatalf("storyboard expired before success TTL: %+v", cached)
			}
			cache.get(context.Background(), "abcdefghijk", 60, description)
			if calls != 1 {
				t.Fatalf("storyboard fetched more than once: %d", calls)
			}
		})
	}
}

func TestChapterRouteReturnsCachedStoryboardWithoutChapters(t *testing.T) {
	cache := newChapterCache()
	var calls int
	cache.load = func(context.Context, string, float64, string) (youtube.VideoTimeline, error) {
		calls++
		return youtube.VideoTimeline{Storyboard: &youtube.Storyboard{Duration: 60, Interval: 10, Count: 6, Width: 160, Height: 90, Columns: 3, Rows: 2, Sheets: []string{"https://i.ytimg.com/sb/abcdefghijk/storyboard3_L0/M0.jpg"}}}, nil
	}
	routes := chapterRoutes{db: &chapterTestDB{video: &models.Video{Type: "video", Duration: 60}}, cache: cache}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/videos/:id/chapters", routes.chapters)
	for range 2 {
		res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/videos/abcdefghijk/chapters", nil))
		if err != nil {
			t.Fatal(err)
		}
		var timeline youtube.VideoTimeline
		decodeErr := json.NewDecoder(res.Body).Decode(&timeline)
		res.Body.Close()
		if res.StatusCode != http.StatusOK || decodeErr != nil || timeline.Chapters == nil || len(timeline.Chapters) != 0 || timeline.Storyboard == nil || timeline.Storyboard.Count != 6 || len(timeline.Storyboard.Sheets) != 1 {
			t.Fatalf("storyboard-only route: status %d, metadata %+v, error %v", res.StatusCode, timeline, decodeErr)
		}
	}
	if calls != 1 {
		t.Fatalf("metadata route did not share cache: %d upstream requests", calls)
	}
}

type chapterTestThumbnailClient struct {
	urls []string
	err  error
}

func (client *chapterTestThumbnailClient) GetChapterThumbnail(_ context.Context, url string) ([]byte, string, error) {
	client.urls = append(client.urls, url)
	return []byte("image bytes"), "image/jpeg", client.err
}

type chapterTestSessions struct {
	database.SessionsClient
}

func (chapterTestSessions) GetSessionReadOnly(context.Context, string) (*models.Session, error) {
	return &models.Session{ID: "session", UserID: null.StringFrom("owner"), ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestStoryboardThumbnailAuthenticatedGetAndHead(t *testing.T) {
	const sheetURL = "https://i.ytimg.com/sb/abcdefghijk/storyboard3_L0/M1.jpg?sigh=signature"
	cache := newChapterCache()
	cache.entries["abcdefghijk"] = chapterCacheEntry{timeline: youtube.VideoTimeline{Storyboard: &youtube.Storyboard{Sheets: []string{"https://i.ytimg.com/unused.jpg", sheetURL}}}, expires: time.Now().Add(time.Hour)}
	client := &chapterTestThumbnailClient{}
	routes := chapterRoutes{cache: cache, thumbnailClient: client}
	sc, err := sessions.New(chapterTestSessions{})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/videos/:id/storyboards/:index/thumbnail", playbackAuthentication(sc), routes.storyboardThumbnail)
	app.Head("/api/videos/:id/storyboards/:index/thumbnail", playbackAuthentication(sc), routes.storyboardThumbnail)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/api/videos/abcdefghijk/storyboards/1/thumbnail?url=https://example.com/ignored.jpg", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "session"})
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil || res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Cache-Control") != "private, max-age=3600" {
			t.Fatalf("bad %s thumbnail response: %+v, %v", method, res, err)
		}
		if method == http.MethodGet && string(body) != "image bytes" || method == http.MethodHead && len(body) != 0 {
			t.Fatalf("bad %s image body: %q", method, body)
		}
	}
	if len(client.urls) != 2 || client.urls[0] != sheetURL || client.urls[1] != sheetURL {
		t.Fatalf("thumbnail must use the cached sheet URL: %v", client.urls)
	}
}

func TestStoryboardThumbnailRejectsInvalidOrUnavailableSheets(t *testing.T) {
	for _, tc := range []struct {
		name       string
		id         string
		index      string
		storyboard *youtube.Storyboard
		expired    bool
		code       int
	}{
		{name: "invalid video", id: "bad", index: "0", code: 400},
		{name: "negative index", id: "abcdefghijk", index: "-1", code: 400},
		{name: "non-numeric index", id: "abcdefghijk", index: "abc", code: 400},
		{name: "overflow index", id: "abcdefghijk", index: "99999999999999999999999", code: 400},
		{name: "missing metadata", id: "lmnopqrstuv", index: "0", code: 404},
		{name: "no storyboard", id: "abcdefghijk", index: "0", code: 404},
		{name: "empty sheets", id: "abcdefghijk", index: "0", storyboard: &youtube.Storyboard{}, code: 404},
		{name: "out of range", id: "abcdefghijk", index: "1", storyboard: &youtube.Storyboard{Sheets: []string{"https://i.ytimg.com/sheet.jpg"}}, code: 404},
		{name: "empty URL", id: "abcdefghijk", index: "0", storyboard: &youtube.Storyboard{Sheets: []string{""}}, code: 404},
		{name: "expired metadata", id: "abcdefghijk", index: "0", storyboard: &youtube.Storyboard{Sheets: []string{"https://i.ytimg.com/sheet.jpg"}}, expired: true, code: 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := newChapterCache()
			expiry := time.Now().Add(time.Hour)
			if tc.expired {
				expiry = time.Now().Add(-time.Second)
			}
			cache.entries["abcdefghijk"] = chapterCacheEntry{timeline: youtube.VideoTimeline{Storyboard: tc.storyboard}, expires: expiry}
			client := &chapterTestThumbnailClient{}
			routes := chapterRoutes{cache: cache, thumbnailClient: client}
			app := fiber.New(fiber.Config{DisableStartupMessage: true})
			app.Get("/api/videos/:id/storyboards/:index/thumbnail", routes.storyboardThumbnail)
			res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/videos/"+tc.id+"/storyboards/"+tc.index+"/thumbnail", nil))
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != tc.code || len(client.urls) != 0 {
				t.Fatalf("status %d, want %d; unexpected image fetches %v", res.StatusCode, tc.code, client.urls)
			}
		})
	}
}

func TestStoryboardThumbnailUpstreamFailure(t *testing.T) {
	cache := newChapterCache()
	cache.entries["abcdefghijk"] = chapterCacheEntry{timeline: youtube.VideoTimeline{Storyboard: &youtube.Storyboard{Sheets: []string{"https://i.ytimg.com/sheet.jpg"}}}, expires: time.Now().Add(time.Hour)}
	routes := chapterRoutes{cache: cache, thumbnailClient: &chapterTestThumbnailClient{err: errors.New("image unavailable")}}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/api/videos/:id/storyboards/:index/thumbnail", routes.storyboardThumbnail)
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/videos/abcdefghijk/storyboards/0/thumbnail", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadGateway {
		t.Fatalf("upstream image failure returned %d", res.StatusCode)
	}
}
