package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/tpot"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

type podcastRouteDatabase struct {
	database.Client
	settings *models.Setting
	job      database.PodcastProcessingJob
	content  database.PodcastTranscriptContent
	segments []database.PodcastSegment
	calls    []string
	jobArgs  [4]string
	cacheKey string
	analysis [4]string
}

func (db *podcastRouteDatabase) GetUserSettings(_ context.Context, userID string) (*models.Setting, error) {
	db.calls = append(db.calls, "settings:"+userID)
	return db.settings, nil
}

func (db *podcastRouteDatabase) GetVideoByID(_ context.Context, id string, _ ...database.VideoQuery) (*models.Video, error) {
	db.calls = append(db.calls, "video:"+id)
	return &models.Video{ID: id, Type: "podcast_episode", Duration: 90, MediaURL: null.StringFrom("https://example.com/episode.mp3")}, nil
}

func (db *podcastRouteDatabase) GetPodcastTranscript(_ context.Context, videoID string) (database.PodcastTranscript, error) {
	db.calls = append(db.calls, "metadata:"+videoID)
	return database.PodcastTranscript{VideoID: videoID, URL: "https://example.com/episode.vtt", MIMEType: "text/vtt"}, nil
}

func (db *podcastRouteDatabase) GetPodcastSourceValidation(_ context.Context, videoID, metadataKey string) (database.PodcastSourceValidation, error) {
	db.calls = append(db.calls, "validation:"+videoID)
	return database.PodcastSourceValidation{VideoID: videoID, MetadataKey: metadataKey, Fingerprint: "cached-source", ValidatedAt: time.Now()}, nil
}

func (db *podcastRouteDatabase) GetPodcastProcessingJob(_ context.Context, videoID, sourceKey, model, prompt string) (database.PodcastProcessingJob, error) {
	db.calls = append(db.calls, "job:"+videoID)
	db.jobArgs = [4]string{videoID, sourceKey, model, prompt}
	if db.job.Status == "" {
		return database.PodcastProcessingJob{}, sql.ErrNoRows
	}
	return db.job, nil
}

func (db *podcastRouteDatabase) GetPodcastTranscriptContent(_ context.Context, videoID, sourceKey string) (database.PodcastTranscriptContent, error) {
	db.calls = append(db.calls, "content:"+videoID)
	db.cacheKey = sourceKey
	if db.content.Source == "" {
		return database.PodcastTranscriptContent{}, sql.ErrNoRows
	}
	return db.content, nil
}

func (db *podcastRouteDatabase) GetPodcastSegmentAnalysis(_ context.Context, videoID, hash, model, prompt string) (database.PodcastSegmentAnalysis, error) {
	db.calls = append(db.calls, "analysis:"+videoID)
	db.analysis = [4]string{videoID, hash, model, prompt}
	return database.PodcastSegmentAnalysis{Status: database.PodcastSegmentReady, Segments: db.segments}, nil
}

func (db *podcastRouteDatabase) EnqueuePodcastProcessingJob(context.Context, string, string, string, string) (database.PodcastProcessingJob, error) {
	db.calls = append(db.calls, "enqueue")
	return database.PodcastProcessingJob{}, errors.New("unexpected processing kickoff")
}

func newPodcastRouteDatabase(t *testing.T, enabled bool, selected []string) *podcastRouteDatabase {
	t.Helper()
	settings := types.SettingsPageProps{PodcastSegments: types.PodcastSegmentSettingsProps{
		Enabled: enabled, SelectedCategories: selected, AvailableCategories: sponsorblock.PodcastAvailableCategories,
	}}
	data, err := settings.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return &podcastRouteDatabase{settings: &models.Setting{UserID: "user", Data: data}}
}

func newPodcastRouteTestApp(t *testing.T, db database.Client, authenticated bool) *fiber.App {
	t.Helper()
	client, err := sessions.New(tvRouteSessions{})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	if authenticated {
		session, err := client.Get(context.Background(), "session")
		if err != nil {
			t.Fatal(err)
		}
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("session", session)
			return c.Next()
		})
	}
	newContext := handler.NewBuilder(db, client, nil, nil)
	toFiber := func(endpoint tpot.Servable[*handler.Context]) fiber.Handler {
		return func(c *fiber.Ctx) error {
			return adaptor.HTTPHandler(endpoint.Handler(newContext(c)))(c)
		}
	}
	app.Get("/api/videos/:id/sponsor-segments", toFiber(PodcastSponsorSegments))
	app.Post("/api/videos/:id/sponsor-segments", toFiber(StartPodcastSponsorSegments))
	return app
}

type podcastRouteTransport func(*http.Request) (*http.Response, error)

func (f podcastRouteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func guardPodcastRouteNetwork(t *testing.T, provider *openrouter.Client) {
	t.Helper()
	previousProvider, previousTransport := openrouter.DefaultClient, http.DefaultTransport
	openrouter.DefaultClient = provider
	var requests atomic.Int32
	http.DefaultTransport = podcastRouteTransport(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	})
	t.Cleanup(func() {
		openrouter.DefaultClient, http.DefaultTransport = previousProvider, previousTransport
		if count := requests.Load(); count != 0 {
			t.Errorf("route made %d outbound requests", count)
		}
	})
}

func testPodcastRouteResponse(t *testing.T, app *fiber.App, method string, wantCode int, wantJSON bool) podcastSegmentsResponse {
	t.Helper()
	response, err := app.Test(httptest.NewRequest(method, "/api/videos/episode/sponsor-segments", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantCode {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, wantCode, body)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		t.Errorf("Cache-Control = %q, want private, no-store", response.Header.Get("Cache-Control"))
	}
	var payload podcastSegmentsResponse
	if wantJSON {
		if !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", response.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload.Segments == nil {
			t.Error("segments must be a JSON array, including when empty")
		}
	}
	return payload
}

func TestPodcastSegmentRoutesRequireAuthenticationAndSettings(t *testing.T) {
	guardPodcastRouteNetwork(t, openrouter.New("test-key", "route-model"))
	for _, tc := range []struct {
		name          string
		method        string
		authenticated bool
		status        int
	}{
		{"GET requires authentication", http.MethodGet, false, http.StatusUnauthorized},
		{"POST requires authentication", http.MethodPost, false, http.StatusUnauthorized},
		{"disabled GET reads no episode or processing state", http.MethodGet, true, http.StatusOK},
		{"disabled POST rejects before loading episode or starting work", http.MethodPost, true, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newPodcastRouteDatabase(t, false, nil)
			payload := testPodcastRouteResponse(t, newPodcastRouteTestApp(t, db, tc.authenticated), tc.method, tc.status, tc.status == http.StatusOK)
			var expected []string
			if tc.authenticated {
				expected = []string{"settings:user"}
			}
			if !slices.Equal(db.calls, expected) {
				t.Errorf("database calls = %v, want %v", db.calls, expected)
			}
			if tc.status == http.StatusOK && (payload.Status != "disabled" || payload.Enabled || payload.PollAfterMS != 0 || len(payload.Segments) != 0) {
				t.Errorf("unexpected disabled response: %+v", payload)
			}
		})
	}
}

func TestPodcastSegmentGETReadsCachedStatusAndFiltersCategories(t *testing.T) {
	guardPodcastRouteNetwork(t, openrouter.New("test-key", "route-model"))
	sponsor := database.PodcastSegment{Category: "sponsor", StartMS: 65000, EndMS: 75000, StartText: "Our sponsor", EndText: "Back to the episode", Reason: "Paid promotion", Brand: "Example"}
	for _, tc := range []struct {
		name       string
		status     string
		phase      string
		failure    string
		duration   int
		cached     database.PodcastTranscriptContent
		selected   []string
		segments   []database.PodcastSegment
		wantSource string
	}{
		{name: "missing job returns idle"},
		{name: "pending job returns poll interval", status: database.PodcastSegmentPending, phase: "queued"},
		{name: "running job reports progress before transcript cache exists", status: database.PodcastSegmentRunning, phase: "transcribing", duration: 90000},
		{name: "running job reports cached generated source and actual duration", status: database.PodcastSegmentRunning, phase: "scanning", duration: 1000, cached: database.PodcastTranscriptContent{Source: "generated", DurationMS: 95000}, wantSource: "generated"},
		{name: "failed job exposes durable error without retrying", status: database.PodcastSegmentFailed, phase: "failed", failure: "transcription_failed"},
		{name: "ready job includes only selected categories", status: database.PodcastSegmentReady, phase: "ready", cached: database.PodcastTranscriptContent{Source: "publisher", DurationMS: 90000}, selected: []string{"sponsor"}, segments: []database.PodcastSegment{sponsor, {Category: "selfpromo", StartMS: 1000, EndMS: 2000}}, wantSource: "publisher"},
		{name: "ready job with no selected categories returns empty array", status: database.PodcastSegmentReady, phase: "ready", selected: []string{}, segments: []database.PodcastSegment{sponsor}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newPodcastRouteDatabase(t, true, tc.selected)
			db.job = database.PodcastProcessingJob{ID: "job", VideoID: "episode", Status: tc.status, Phase: tc.phase, Error: tc.failure, DurationMS: tc.duration, TranscriptHash: "current-transcript", Model: "route-model", PromptVersion: "current-prompt"}
			db.content, db.segments = tc.cached, tc.segments
			payload := testPodcastRouteResponse(t, newPodcastRouteTestApp(t, db, true), http.MethodGet, http.StatusOK, true)
			wantStatus, wantDuration := tc.status, tc.duration
			if wantStatus == "" {
				wantStatus = "idle"
			}
			if tc.cached.Source != "" {
				wantDuration = tc.cached.DurationMS
			}
			wantPoll := 0
			if tc.status == database.PodcastSegmentPending || tc.status == database.PodcastSegmentRunning {
				wantPoll = 2000
			}
			if !payload.Enabled || payload.Status != wantStatus || payload.Phase != tc.phase || payload.Error != tc.failure || payload.Source != tc.wantSource || payload.DurationMS != wantDuration || payload.PollAfterMS != wantPoll {
				t.Errorf("unexpected cached status response: %+v", payload)
			}
			var wantSegments []podcastSegmentResponse
			if slices.Contains(tc.selected, "sponsor") {
				wantSegments = []podcastSegmentResponse{{Category: "sponsor", StartMS: 65000, EndMS: 75000, StartTime: "1:05", EndTime: "1:15", StartText: sponsor.StartText, EndText: sponsor.EndText, Reason: sponsor.Reason, Brand: sponsor.Brand, Skippable: true}}
			}
			if !slices.Equal(payload.Segments, wantSegments) {
				t.Errorf("segments = %+v, want %+v", payload.Segments, wantSegments)
			}
			wantCalls := []string{"settings:user", "video:episode", "metadata:episode", "validation:episode", "job:episode"}
			if tc.status != "" {
				wantCalls = append(wantCalls, "content:episode")
				if db.cacheKey == "" {
					t.Error("transcript content lookup did not specify the current source key")
				}
			}
			if tc.status == database.PodcastSegmentReady {
				wantCalls = append(wantCalls, "analysis:episode")
				if db.analysis != [4]string{"episode", "current-transcript", "route-model", "current-prompt"} {
					t.Errorf("analysis lookup used stale inputs: %v", db.analysis)
				}
			}
			if !slices.Equal(db.calls, wantCalls) {
				t.Errorf("database calls = %v, want cache-only reads %v", db.calls, wantCalls)
			}
			if db.jobArgs[0] != "episode" || db.jobArgs[1] == "" || db.jobArgs[2] != "route-model" || db.jobArgs[3] == "" {
				t.Errorf("job lookup did not select current processing inputs: %v", db.jobArgs)
			}
		})
	}
}

func TestPodcastSegmentPOSTWithoutProviderReturnsUnavailable(t *testing.T) {
	guardPodcastRouteNetwork(t, nil)
	db := newPodcastRouteDatabase(t, true, []string{"sponsor"})
	payload := testPodcastRouteResponse(t, newPodcastRouteTestApp(t, db, true), http.MethodPost, http.StatusOK, true)
	if !payload.Enabled || payload.Status != database.PodcastSegmentUnavailable || payload.Error != "provider_disabled" || payload.PollAfterMS != 0 || len(payload.Segments) != 0 {
		t.Errorf("unexpected unavailable response: %+v", payload)
	}
	assertPodcastRouteLocalReads(t, db.calls)
}

func TestPodcastSegmentPOSTWithoutProviderRetainsReadyCache(t *testing.T) {
	guardPodcastRouteNetwork(t, nil)
	db := newPodcastRouteDatabase(t, true, []string{"sponsor"})
	db.job = database.PodcastProcessingJob{VideoID: "episode", Status: database.PodcastSegmentReady, Phase: "ready", TranscriptHash: "cached-hash", Model: openrouter.DefaultModel, PromptVersion: "cached-prompt"}
	db.content = database.PodcastTranscriptContent{Source: "generated", DurationMS: 95000}
	db.segments = []database.PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 2000}}
	payload := testPodcastRouteResponse(t, newPodcastRouteTestApp(t, db, true), http.MethodPost, http.StatusOK, true)
	if !payload.Enabled || payload.Status != database.PodcastSegmentReady || payload.Error != "" || payload.Source != "generated" || payload.DurationMS != 95000 || len(payload.Segments) != 1 || !payload.Segments[0].Skippable {
		t.Errorf("cached ready result was lost when provider became unavailable: %+v", payload)
	}
	assertPodcastRouteLocalReads(t, db.calls)
}

func assertPodcastRouteLocalReads(t *testing.T, calls []string) {
	t.Helper()
	if len(calls) == 0 || calls[0] != "settings:user" || !slices.Contains(calls, "video:episode") || !slices.Contains(calls, "metadata:episode") {
		t.Fatalf("local episode reads must follow user settings: %v", calls)
	}
	for _, call := range calls[1:] {
		if !slices.Contains([]string{"video:episode", "metadata:episode", "validation:episode", "job:episode", "content:episode", "analysis:episode"}, call) {
			t.Errorf("unexpected call %q, want local cache reads without enqueue", call)
		}
	}
}
