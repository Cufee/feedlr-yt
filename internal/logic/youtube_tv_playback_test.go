package logic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type tvPlaybackHarness struct {
	store    *tvMetadataTestStore
	service  *YouTubeTVSyncService
	runtime  *tvSyncRuntime
	account  *database.YouTubeTVSyncAccount
	session  *lounge.Session
	mu       sync.Mutex
	commands []string
}

func newTVPlaybackHarness(t *testing.T, saved int, commandStatus int) *tvPlaybackHarness {
	t.Helper()
	h := &tvPlaybackHarness{store: newTVMetadataTestStore(), runtime: newTVSyncRuntime(false, nil), account: &database.YouTubeTVSyncAccount{UserID: "user"}, session: &lounge.Session{SID: "sid", GSessionID: "gsid"}}
	h.store.videos["video"] = &models.Video{ID: "video"}
	h.store.mockTVSyncStore.account = h.account
	h.store.views["user:video"] = &models.View{UserID: "user", VideoID: "video", Progress: int64(saved)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		h.mu.Lock()
		h.commands = append(h.commands, r.FormValue("req0__sc"))
		h.mu.Unlock()
		w.WriteHeader(commandStatus)
	}))
	t.Cleanup(server.Close)
	h.service = &YouTubeTVSyncService{db: h.store, lounge: lounge.NewClientWithBaseURL(server.Client(), server.URL)}
	return h
}

func (h *tvPlaybackHarness) event(t *testing.T, kind string, fields map[string]any) {
	t.Helper()
	if err := h.service.processEvent(context.Background(), h.account, h.session, h.runtime, lounge.Event{Type: kind, Args: []any{fields}}); err != nil {
		t.Fatal(err)
	}
}

func (h *tvPlaybackHarness) expectProgress(t *testing.T, want int64) {
	t.Helper()
	views, err := h.store.GetUserViews(context.Background(), "user", "video")
	if err != nil || len(views) != 1 {
		t.Fatalf("views: %v %v", views, err)
	}
	if got := views[0].Progress; got != want {
		t.Errorf("saved progress = %d; want %d", got, want)
	}
}

// Each case asserts preservation of progress or completion of a resume/flush.
func TestTVPlayback(t *testing.T) {
	t.Run("EmptyNowPlayingAfterHoursUsesPreviousVideo", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 1200, http.StatusOK)
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "1"})
		h.runtime.videoRuntime("video").lastProgressWrite = time.Now().Add(-3 * time.Hour)
		h.event(t, "nowPlaying", map[string]any{"videoId": "", "currentTime": 0})
		h.expectProgress(t, 1200)
		video, state := h.runtime.currentPlaybackSnapshot()
		if video != "" || state != "" {
			t.Fatalf("empty snapshot retained video=%q state=%q", video, state)
		}
	})
	t.Run("MissingStateOverridesKnownUnstartedState", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 1200, http.StatusOK)
		h.runtime.setCurrentVideo("video")
		h.runtime.setCurrentPlaybackState("-1")
		h.runtime.videoRuntime("video").lastState = "-1"
		h.event(t, "onStateChange", map[string]any{"currentTime": 0})
		h.expectProgress(t, 1200)
	})
	t.Run("ReconnectSuppressionAcceptsSecondZero", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 1200, http.StatusOK)
		h.account.LastVideoID = null.StringFrom("video")
		h.account.LastDisconnectAt = null.TimeFrom(time.Now().Add(-30 * time.Second))
		for range 2 {
			h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "1"})
		}
		h.expectProgress(t, 1200)
	})
	for _, tc := range []struct {
		name   string
		status int
	}{{"SuccessfulSeekFollowedByPreSeekZero", http.StatusOK}, {"FailedSeekConsumesResumeAndNextZero", http.StatusInternalServerError}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTVPlaybackHarness(t, 1200, tc.status)
			for range 2 {
				h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "1"})
			}
			h.expectProgress(t, 1200)
			h.mu.Lock()
			defer h.mu.Unlock()
			t.Logf("commands=%v, resumeApplied=%v", h.commands, h.runtime.resumeAppliedForCurrentVideo())
		})
	}
	t.Run("VideoAndPositionInSeparateEventsSkipResume", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 1200, http.StatusOK)
		h.event(t, "onStateChange", map[string]any{"videoId": "video", "state": "1"})
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "1"})
		h.expectProgress(t, 1200)
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.commands) != 2 || h.commands[0] != "getNowPlaying" || h.commands[1] != "seekTo" {
			t.Fatalf("unexpected startup commands: %v", h.commands)
		}
	})
	t.Run("CachedVideoEndInsideWriteIntervalIsDropped", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 295, http.StatusOK)
		h.runtime.setCurrentVideo("video")
		h.runtime.setCurrentPlaybackState("1")
		h.runtime.markResumeAppliedForCurrentVideo()
		h.runtime.videoRuntime("video").lastState = "1"
		h.runtime.videoRuntime("video").lastProgressWrite = time.Now()
		h.event(t, "onStateChange", map[string]any{"videoId": "video", "currentTime": 300, "duration": 300, "state": "0"})
		h.expectProgress(t, 300)
		t.Logf("active video after dropped final progress=%q", h.runtime.currentVideo())
	})
	t.Run("EndedSnapshotStillTriggersResumeSeek", func(t *testing.T) {
		h := newTVPlaybackHarness(t, 1200, http.StatusOK)
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "0"})
		h.mu.Lock()
		defer h.mu.Unlock()
		if len(h.commands) != 0 {
			t.Errorf("commands for ended playback=%v; want none", h.commands)
		}
		if h.runtime.currentVideo() != "" {
			t.Errorf("ended video was not cleared")
		}
	})
}

type tvPlaybackReadFailure struct{ *mockTVSyncStore }

func (s *tvPlaybackReadFailure) GetYouTubeTVSyncAccountByUserID(context.Context, string) (*database.YouTubeTVSyncAccount, error) {
	return nil, fmt.Errorf("database temporarily unavailable")
}

func TestTVPlaybackTokenRefreshReadErrorMustNotPanic(t *testing.T) {
	crypto := newYouTubeSyncCrypto("review-secret")
	token, err := crypto.Encrypt([]byte("expired"), "user")
	if err != nil {
		t.Fatal(err)
	}
	account := &database.YouTubeTVSyncAccount{UserID: "user", ScreenID: "screen", LoungeTokenEnc: token, EncSecretHash: crypto.secretHash}
	store := &tvPlaybackReadFailure{&mockTVSyncStore{account: account}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pairing/get_lounge_token_batch" {
			_, _ = fmt.Fprint(w, `{"screens":[{"screenId":"screen","loungeToken":"fresh"}]}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	s := &YouTubeTVSyncService{db: store, crypto: crypto, lounge: lounge.NewClientWithBaseURL(server.Client(), server.URL)}
	defer func() {
		if p := recover(); p != nil {
			t.Errorf("token refresh database failure panics instead of returning error: %v", p)
		}
	}()
	if err := s.connectAndRunOnce(context.Background(), account); err == nil {
		t.Error("expected database error")
	}
}

func TestTVPlaybackInitialBindPlaybackIsNotLost(t *testing.T) {
	crypto := newYouTubeSyncCrypto("review-secret")
	token, err := crypto.Encrypt([]byte("valid"), "user")
	if err != nil {
		t.Fatal(err)
	}
	account := &database.YouTubeTVSyncAccount{UserID: "user", ScreenID: "screen", LoungeTokenEnc: token, EncSecretHash: crypto.secretHash}
	store := &mockTVSyncStore{account: account}
	var mu sync.Mutex
	var subscribeAID string
	var commands int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Query().Get("RID") == "1" {
			chunk := `[[1,["c","sid"]],[2,["S","gsid"]],[3,["nowPlaying",{"videoId":"video","currentTime":"1200","state":"1"}]]]`
			_, _ = fmt.Fprintf(w, "%d\n%s\n", len(chunk)+1, chunk)
			return
		}
		if r.Method == http.MethodGet {
			mu.Lock()
			subscribeAID = r.URL.Query().Get("AID")
			mu.Unlock()
			chunk := `[[4,["noop"]]]`
			_, _ = fmt.Fprintf(w, "%d\n%s\n", len(chunk)+1, chunk)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		mu.Lock()
		commands++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	s := &YouTubeTVSyncService{db: store, crypto: crypto, lounge: lounge.NewClientWithBaseURL(server.Client(), server.URL), noEventTimeout: 50 * time.Millisecond, watchdogPollInterval: 5 * time.Millisecond, nowPlayingPollInterval: 5 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = s.connectAndRunOnce(ctx, account)
	updated, err := store.GetYouTubeTVSyncAccountByUserID(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !updated.LastVideoID.Valid {
		t.Errorf("initial playback was lost: LastVideoID missing, AID=%s, refresh commands=%d", subscribeAID, commands)
	}
}

func TestTVPlaybackConfirmedResumeAllowsRewindAndRestart(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	playing := func(second int) {
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": second, "state": "1"})
	}
	playing(0)
	playing(0)
	h.expectProgress(t, 1200)
	if h.runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("HTTP success was treated as playback confirmation")
	}
	playing(1201)
	h.expectProgress(t, 1201)
	playing(30) // A deliberate backward seek inside the write interval.
	h.event(t, "onStateChange", map[string]any{"state": "2"})
	h.expectProgress(t, 30)
	playing(0) // A restart does not persist a teardown-like zero.
	h.expectProgress(t, 30)
	playing(1)
	h.event(t, "onStateChange", map[string]any{"state": "2", "currentTime": 2})
	h.expectProgress(t, 2)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) != 1 || h.commands[0] != "seekTo" {
		t.Fatalf("resume was repeated after confirmation: %v", h.commands)
	}
}

func TestTVPlaybackReconnectWaitsForAdvanceThenResumes(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	h.account.LastVideoID = null.StringFrom("video")
	h.account.LastDisconnectAt = null.TimeFrom(time.Now())
	for range 3 {
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "1"})
	}
	h.expectProgress(t, 1200)
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1, "state": "1"})
	h.expectProgress(t, 1200)
	h.event(t, "onStateChange", map[string]any{"currentTime": 1202, "state": "2"})
	h.expectProgress(t, 1202)
	if !h.runtime.resumeAppliedForCurrentVideo() {
		t.Fatal("paused target did not confirm resume")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) != 2 || h.commands[0] != "getNowPlaying" || h.commands[1] != "seekTo" {
		t.Fatalf("unexpected reconnect commands: %v", h.commands)
	}
}

func TestTVPlaybackResumeRetriesAreBoundedAndRecover(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusInternalServerError)
	playing := func(second int) {
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": second, "state": "1"})
	}
	playing(0)
	for range 5 {
		h.runtime.videoRuntime("video").lastResumeAttempt = time.Now().Add(-tvSyncResumeRetryInterval)
		playing(0)
	}
	h.expectProgress(t, 1200)
	h.mu.Lock()
	if len(h.commands) != tvSyncResumeMaxAttempts {
		t.Errorf("resume commands=%d, want %d", len(h.commands), tvSyncResumeMaxAttempts)
	}
	h.mu.Unlock()
	h.runtime.videoRuntime("video").resumeStartedAt = time.Now().Add(-tvSyncResumeConfirmationTimeout)
	playing(0)
	h.expectProgress(t, 1200)
	playing(5) // The TV ignored the command but is now demonstrably playing.
	h.expectProgress(t, 5)
}

func TestTVPlaybackFlushesOnVideoChangeAndDisconnect(t *testing.T) {
	for _, ending := range []string{"empty", "new", "disconnect", "ended-zero"} {
		t.Run(ending, func(t *testing.T) {
			h := newTVPlaybackHarness(t, 1200, http.StatusOK)
			h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "1"})
			h.event(t, "onStateChange", map[string]any{"currentTime": 1205})
			h.expectProgress(t, 1200)
			switch ending {
			case "empty":
				h.event(t, "nowPlaying", map[string]any{})
			case "new":
				h.event(t, "nowPlaying", map[string]any{"videoId": "another-video", "state": "-1"})
			case "ended-zero":
				h.event(t, "onStateChange", map[string]any{"currentTime": 0, "state": "0"})
			case "disconnect":
				if err := h.service.processEvent(context.Background(), h.account, h.session, h.runtime, lounge.Event{Type: "loungeScreenDisconnected"}); err == nil {
					t.Fatal("expected disconnect")
				}
			}
			h.expectProgress(t, 1205)
		})
	}
}

func TestTVPlaybackHeartbeatsDoNotRefreshPlaybackOrAllowUnownedDeltas(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "1"})
	h.runtime.lastPlaybackAt = time.Now().Add(-3 * time.Hour)
	h.runtime.markEvent(time.Now())
	h.event(t, "noop", nil)
	h.event(t, "onStateChange", map[string]any{"currentTime": 20})
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "1"})
	if h.runtime.playbackFresh(time.Now()) {
		t.Fatal("heartbeat or frozen snapshot refreshed stale playback")
	}
	h.expectProgress(t, 1200)
}

func TestTVPlaybackUnchangedPausedSnapshotDoesNotOverwriteWebProgress(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "1"})
	h.event(t, "onStateChange", map[string]any{"currentTime": 1200, "state": "2"})
	if _, err := UpdateViewProgress(context.Background(), h.store, "user", "video", 1500); err != nil {
		t.Fatal(err)
	}
	h.runtime.videoRuntime("video").lastProgressWrite = time.Now().Add(-time.Minute)
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1200, "state": "2"})
	h.expectProgress(t, 1500)
}

type tvPlaybackViewReadFailure struct{ *tvMetadataTestStore }

func (s *tvPlaybackViewReadFailure) GetUserViews(context.Context, string, ...string) ([]*models.View, error) {
	return nil, fmt.Errorf("read unavailable")
}

func TestTVPlaybackResumeReadFailurePreservesProgressAndRetries(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	h.service.db = &tvPlaybackViewReadFailure{h.store}
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 5, "state": "1"})
	h.expectProgress(t, 1200)
	h.service.db = h.store
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 6, "state": "1"})
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 1201, "state": "1"})
	h.expectProgress(t, 1201)
}

func TestTVPlaybackSameVideoResumesAfterHoursIdle(t *testing.T) {
	h := newTVPlaybackHarness(t, 1200, http.StatusOK)
	playing := func(second int) {
		h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": second, "state": "1"})
	}
	playing(1200)
	playing(0) // Teardown snapshot without a disconnect event.
	h.runtime.lastPlaybackAt = time.Now().Add(-3 * time.Hour)
	playing(0) // Frozen background snapshots cannot trigger another seek.
	h.expectProgress(t, 1200)
	playing(1) // TV starts the same video hours later.
	playing(2)
	h.expectProgress(t, 1200)
	playing(1201)
	h.expectProgress(t, 1201)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.commands) != 2 || h.commands[0] != "getNowPlaying" || h.commands[1] != "seekTo" {
		t.Fatalf("same-video resume failed: %v", h.commands)
	}
}
