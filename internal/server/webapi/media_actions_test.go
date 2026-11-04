package webapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v3"
)

func TestAPIMarkWatchedSupportsUnknownDurationAndReset(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration int64
	}{
		{name: "unknown", duration: 0},
		{name: "negative_unknown", duration: -1},
		{name: "known", duration: 120},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			video, err := f.db.GetVideoByID(ctx, "pe_first")
			if err != nil {
				t.Fatal(err)
			}
			video.Duration = tc.duration
			if err := f.db.UpsertVideos(ctx, video); err != nil {
				t.Fatal(err)
			}
			res, value := f.request(t, http.MethodPut, "/watch-later/pe_first", `{"present":true}`, f.session, "")
			if res.StatusCode != http.StatusOK {
				t.Fatalf("save for later: %d %#v", res.StatusCode, value)
			}
			patch := func(body string) {
				t.Helper()
				res, value := f.request(t, http.MethodPatch, "/media/pe_first/state", body, f.session, "")
				if res.StatusCode != http.StatusNoContent {
					t.Fatalf("state %s: %d %#v", body, res.StatusCode, value)
				}
			}
			assertState := func(progress int64, hidden, completed bool) {
				t.Helper()
				views, err := f.db.GetUserViews(ctx, "viewer", "pe_first")
				if err != nil || len(views) != 1 {
					t.Fatalf("stored state: %#v %v", views, err)
				}
				if views[0].Progress != progress || views[0].Hidden.Bool != hidden {
					t.Fatalf("stored progress/visibility: %#v", views[0])
				}
				res, value := f.request(t, http.MethodGet, "/media/pe_first", "", f.session, "")
				if res.StatusCode != http.StatusOK {
					t.Fatalf("detail: %d %#v", res.StatusCode, value)
				}
				f.check(t, value, "MediaDetail")
				item := value.(map[string]any)["item"].(map[string]any)
				if item["progressSeconds"] != float64(progress) || item["hidden"] != hidden || item["completed"] != completed {
					t.Fatalf("detail progress/visibility/completion: %#v", item)
				}
				// Unknown duration must remain unknown, and must not count as
				// full playback or automatically remove the saved item.
				if video.Duration <= 0 && (item["durationSeconds"] != nil || item["inWatchLater"] != true) {
					t.Fatalf("unknown-duration metadata or membership changed: %#v", item)
				}
			}
			assertWatchedFeed := func(progress int64) {
				t.Helper()
				for _, path := range []string{"/history", "/feed"} {
					res, value := f.request(t, http.MethodGet, path, "", f.session, "")
					if res.StatusCode != http.StatusOK {
						t.Fatalf("%s: %d %#v", path, res.StatusCode, value)
					}
					key, schema := "items", "MediaList"
					if path == "/feed" {
						key, schema = "watched", "Feed"
					}
					f.check(t, value, schema)
					found := false
					for _, raw := range value.(map[string]any)[key].([]any) {
						item := raw.(map[string]any)
						if item["id"] == video.ID {
							found = true
							if item["progressSeconds"] != float64(progress) {
								t.Fatalf("%s lost watched progress: %#v", path, item)
							}
						}
					}
					if !found {
						t.Fatalf("%s omitted watched item: %#v", path, value)
					}
				}
			}

			patch(`{"completed":true}`)
			wantProgress := max(tc.duration+1, 1)
			assertState(wantProgress, false, tc.duration > 0)
			assertWatchedFeed(wantProgress)
			patch(`{"hidden":true}`)
			assertState(wantProgress, true, tc.duration > 0)
			patch(`{"hidden":false}`)
			assertState(wantProgress, false, tc.duration > 0)
			if tc.duration <= 0 {
				// Learning the duration must not move a manually watched item
				// back into the unwatched feed, or replace its saved position.
				video.Duration = 120
				if err := f.db.UpsertVideos(ctx, video); err != nil {
					t.Fatal(err)
				}
				assertState(wantProgress, false, false)
				assertWatchedFeed(wantProgress)
			}
			patch(`{"progressSeconds":45}`)
			assertState(45, false, false)
			patch(`{"completed":false}`)
			assertState(0, false, false)
		})
	}
}

func TestTVJSONRoutesAuthenticateValidateAndReportOffline(t *testing.T) {
	f := newFixture(t)
	previous := logic.DefaultYouTubeTVSync
	logic.DefaultYouTubeTVSync = nil
	t.Cleanup(func() { logic.DefaultYouTubeTVSync = previous })
	for _, tc := range []struct {
		method, path, body, session string
		status                      int
	}{
		{"GET", "/tv/status", "", "", 401},
		{"POST", "/media/abcdefghijk/tv", `{"position":0}`, "", 401},
		{"GET", "/tv/status", "", f.session, 200},
		{"POST", "/media/abcdefghijk/tv", `{}`, f.session, 422},
		{"POST", "/media/abcdefghijk/tv", `{"position":null}`, f.session, 422},
		{"POST", "/media/abcdefghijk/tv", `{"position":"10"}`, f.session, 400},
		{"POST", "/media/abcdefghijk/tv", `{"position":`, f.session, 400},
		{"POST", "/media/abcdefghijk/tv", `{"position":-1}`, f.session, 422},
		{"POST", "/media/abcdefghijk/tv", `{"position":12.5}`, f.session, 409},
	} {
		res, value := f.request(t, tc.method, tc.path, tc.body, tc.session, "")
		if res.StatusCode != tc.status {
			t.Fatalf("%s %s %s: %d %#v", tc.method, tc.path, tc.body, res.StatusCode, value)
		}
		if tc.status == 200 {
			f.check(t, value, "TVPlayerStatus")
			if value.(map[string]any)["online"] != false {
				t.Fatalf("nil TV service reported online: %#v", value)
			}
		} else {
			f.check(t, value, "Error")
		}
		if res.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatal("TV availability was cacheable")
		}
	}
	// Invalid inputs fail before a configured service can access storage/network.
	logic.DefaultYouTubeTVSync = &logic.YouTubeTVSyncService{}
	for _, tc := range []struct{ id, body string }{{"invalid", `{"position":0}`}, {"abcdefghijk", `{"position":2147483648}`}} {
		res, value := f.request(t, "POST", "/media/"+tc.id+"/tv", tc.body, f.session, "")
		if res.StatusCode != 422 {
			t.Fatalf("invalid TV request: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "Error")
	}
}

func TestPodcastJSONRoutesGateSettingsAndUseCacheWithoutProviderTraffic(t *testing.T) {
	f := newFixture(t)
	previousProvider, previousTransport := openrouter.DefaultClient, http.DefaultTransport
	openrouter.DefaultClient = nil
	var requests atomic.Int32
	http.DefaultTransport = testTransport(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("network prohibited")
	})
	t.Cleanup(func() {
		openrouter.DefaultClient, http.DefaultTransport = previousProvider, previousTransport
		if requests.Load() != 0 {
			t.Errorf("cache/settings route made %d network requests", requests.Load())
		}
	})
	for _, method := range []string{"GET", "POST"} {
		res, value := f.request(t, method, "/media/pe_first/segments", "", "", "")
		if res.StatusCode != 401 {
			t.Fatalf("unauthenticated analysis: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "Error")
	}
	res, value := f.request(t, "PATCH", "/settings", `{"podcastSegments":{"enabled":false}}`, f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("disable: %d %#v", res.StatusCode, value)
	}
	// A nonexistent episode is deliberately used: disabled requests must not load it.
	res, value = f.request(t, "GET", "/media/missing/segments", "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("disabled cache: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "PodcastAnalysis")
	if value.(map[string]any)["status"] != "disabled" || value.(map[string]any)["enabled"] != false {
		t.Fatalf("disabled response: %#v", value)
	}
	res, value = f.request(t, "POST", "/media/missing/segments", "", f.session, "")
	if res.StatusCode != 403 {
		t.Fatalf("disabled kickoff: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "Error")
	res, value = f.request(t, "PATCH", "/settings", `{"podcastSegments":{"enabled":true}}`, f.session, "")
	if res.StatusCode != 204 {
		t.Fatal(res.StatusCode)
	}
	for _, method := range []string{"GET", "POST"} {
		res, value = f.request(t, method, "/media/missing/segments", "", f.session, "")
		if res.StatusCode != 404 {
			t.Fatalf("missing episode: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "Error")
	}
	for _, method := range []string{"GET", "POST"} {
		res, value = f.request(t, method, "/media/pe_first/segments", "", f.session, "")
		if res.StatusCode != 200 {
			t.Fatalf("uncached analysis: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "PodcastAnalysis")
		want := "idle"
		if method == "POST" {
			want = "unavailable"
		}
		if value.(map[string]any)["status"] != want {
			t.Fatalf("provider-disabled response: %#v", value)
		}
	}
}

// Real sessions/settings wrap a deterministic cache to verify the HTTP contract
// for progressive analysis without kicking off workers or provider requests.
type segmentCacheDatabase struct {
	database.Client
	status, failure, source string
	segments                []database.PodcastSegment
}

func (d *segmentCacheDatabase) GetPodcastSourceValidation(_ context.Context, videoID, metadataKey string) (database.PodcastSourceValidation, error) {
	return database.PodcastSourceValidation{VideoID: videoID, MetadataKey: metadataKey, Fingerprint: "cached-source", ValidatedAt: time.Now()}, nil
}
func (d *segmentCacheDatabase) GetPodcastTranscriptContent(context.Context, string, string) (database.PodcastTranscriptContent, error) {
	if d.source == "" {
		return database.PodcastTranscriptContent{}, sql.ErrNoRows
	}
	return database.PodcastTranscriptContent{Source: d.source, DurationMS: 90000}, nil
}
func (d *segmentCacheDatabase) GetPodcastProcessingJob(context.Context, string, string, string, string) (database.PodcastProcessingJob, error) {
	return database.PodcastProcessingJob{ID: "job", Status: d.status, Phase: "scanning", Error: d.failure, AnalysisID: "analysis", TranscriptHash: "hash", Model: "model", PromptVersion: "prompt", DurationMS: 90000}, nil
}
func (d *segmentCacheDatabase) GetPodcastSegmentAnalysis(context.Context, string, string, string, string) (database.PodcastSegmentAnalysis, error) {
	return database.PodcastSegmentAnalysis{ID: "analysis", Segments: d.segments}, nil
}
func (d *segmentCacheDatabase) GetPodcastSegmentAnalysisInputs(context.Context, string) ([]byte, error) {
	return []byte(`{"transcript_source":"generated"}`), nil
}
func TestPodcastJSONProgressiveCacheFiltersDeduplicatesAndInvalidatesIntervals(t *testing.T) {
	f := newFixture(t)
	res, value := f.request(t, "PATCH", "/settings", `{"podcastSegments":{"enabled":true,"categories":["sponsor"]}}`, f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("settings: %d %#v", res.StatusCode, value)
	}
	segment := database.PodcastSegment{Category: "sponsor", StartMS: 65000, EndMS: 75000, Brand: "Example", StartText: "Sponsor", EndText: "Back"}
	db := &segmentCacheDatabase{Client: f.db, segments: []database.PodcastSegment{segment, segment, {Category: "selfpromo", StartMS: 1, EndMS: 1000}}}
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	f.app = fiber.New()
	Register(f.app, db, sc, nil)
	for _, tc := range []struct{ status, failure, source string }{
		{database.PodcastSegmentPending, "", ""},
		{database.PodcastSegmentRunning, "", "generated_chunk"},
		{database.PodcastSegmentFailed, "transcription_failed", "generated"},
		{database.PodcastSegmentReady, "", "publisher"},
		{database.PodcastSegmentFailed, "source_changed", "generated"},
		{database.PodcastSegmentFailed, "source_validation_failed", "generated"},
		{database.PodcastSegmentFailed, "configuration_changed", "generated"},
	} {
		db.status, db.failure, db.source = tc.status, tc.failure, tc.source
		res, value := f.request(t, "GET", "/media/pe_first/segments", "", f.session, "")
		if res.StatusCode != 200 {
			t.Fatalf("cached response: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "PodcastAnalysis")
		payload := value.(map[string]any)
		if payload["status"] != tc.status || payload["duration_ms"] != float64(90000) || payload["transcript_ready"] != (tc.source == "generated" || tc.source == "publisher") {
			t.Fatalf("analysis state lost: %#v", payload)
		}
		_, polls := payload["poll_after_ms"]
		if polls != (tc.status == database.PodcastSegmentPending || tc.status == database.PodcastSegmentRunning) {
			t.Fatalf("wrong polling state: %#v", payload)
		}
		wantSegments := 1
		if tc.failure == "source_changed" || tc.failure == "source_validation_failed" || tc.failure == "configuration_changed" {
			wantSegments = 0
		}
		if got := len(payload["segments"].([]any)); got != wantSegments || payload["has_segments"] != (got > 0) {
			t.Fatalf("unsafe intervals: %#v", payload)
		}
	}
}
