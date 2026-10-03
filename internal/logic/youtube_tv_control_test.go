package logic

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
)

func newTVControlHarness(t *testing.T, commandStatus int) (*YouTubeTVSyncService, *mockTVSyncStore, *tvSyncConnection, <-chan url.Values) {
	t.Helper()
	commands := make(chan url.Values, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		commands <- r.PostForm
		w.WriteHeader(commandStatus)
	}))
	t.Cleanup(server.Close)
	store := &mockTVSyncStore{account: &database.YouTubeTVSyncAccount{UserID: "user", ScreenID: "screen", ScreenName: "Living Room", SyncEnabled: true, ConnectionState: tvSyncStateConnected}}
	service := &YouTubeTVSyncService{db: store, lounge: lounge.NewClientWithBaseURL(server.Client(), server.URL)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	connection := &tvSyncConnection{ctx: ctx, cancel: cancel, session: &lounge.Session{ScreenID: "screen", SID: "sid", GSessionID: "gsid"}, runtime: newTVSyncRuntime(false, nil)}
	service.registerTVConnection("user", connection)
	return service, store, connection, commands
}

func TestTVPlayerAvailability(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*YouTubeTVSyncService, *mockTVSyncStore, *tvSyncConnection)
		online bool
	}{
		{"cloud session alone", func(_ *YouTubeTVSyncService, _ *mockTVSyncStore, c *tvSyncConnection) { c.runtime.screenOnline = false }, false},
		{"idle receiver online", func(_ *YouTubeTVSyncService, _ *mockTVSyncStore, _ *tvSyncConnection) {}, true},
		{"stale events", func(_ *YouTubeTVSyncService, _ *mockTVSyncStore, c *tvSyncConnection) {
			c.runtime.lastEventAt = time.Now().Add(-2 * time.Minute)
		}, false},
		{"canceled stream", func(_ *YouTubeTVSyncService, _ *mockTVSyncStore, c *tvSyncConnection) { c.cancel() }, false},
		{"disabled", func(_ *YouTubeTVSyncService, db *mockTVSyncStore, _ *tvSyncConnection) {
			db.account.SyncEnabled = false
		}, false},
		{"unpaired", func(_ *YouTubeTVSyncService, db *mockTVSyncStore, _ *tvSyncConnection) { db.account = nil }, false},
		{"repaired", func(_ *YouTubeTVSyncService, db *mockTVSyncStore, _ *tvSyncConnection) {
			db.account.ScreenID = "other-screen"
		}, false},
		{"persisted connected but no live worker", func(s *YouTubeTVSyncService, _ *mockTVSyncStore, c *tvSyncConnection) {
			s.removeTVConnection("user", c)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db, c, commands := newTVControlHarness(t, http.StatusOK)
			c.runtime.screenOnline = true
			tc.change(s, db, c)
			status, err := s.PlayerStatus(context.Background(), "user")
			if err != nil || status.Online != tc.online {
				t.Fatalf("status = %+v, %v; want online %v", status, err, tc.online)
			}
			if tc.online && status.ScreenName != "Living Room" {
				t.Fatalf("wrong screen name: %q", status.ScreenName)
			}
			if !tc.online && status.ScreenName != "" {
				t.Fatal("offline status exposed a target")
			}
			if err := s.SendVideo(context.Background(), "user", "abcdefghijk", 123.5); !tc.online && !errors.Is(err, ErrTVOffline) || tc.online && err != nil {
				t.Fatalf("send error: %v", err)
			}
			if !tc.online && len(commands) != 0 {
				t.Fatal("offline send issued a command")
			}
		})
	}
}

func TestTVSendUsesUserSessionAndPosition(t *testing.T) {
	s, _, c, commands := newTVControlHarness(t, http.StatusOK)
	c.runtime.screenOnline = true
	if status, err := s.PlayerStatus(context.Background(), "someone-else"); err != nil || status.Online {
		t.Fatalf("other user status = %+v, %v", status, err)
	}
	if err := s.SendVideo(context.Background(), "someone-else", "abcdefghijk", 12); !errors.Is(err, ErrTVOffline) {
		t.Fatalf("other user send: %v", err)
	}
	for _, position := range []float64{-1, math.NaN(), math.Inf(1), math.MaxFloat64} {
		if err := s.SendVideo(context.Background(), "user", "abcdefghijk", position); !errors.Is(err, ErrInvalidTVVideo) {
			t.Fatalf("position %v: %v", position, err)
		}
	}
	if err := s.SendVideo(context.Background(), "user", "pe_podcast-episode", 0); !errors.Is(err, ErrInvalidTVVideo) {
		t.Fatalf("podcast send: %v", err)
	}
	if len(commands) != 0 {
		t.Fatal("invalid send reached the TV")
	}
	if err := s.SendVideo(context.Background(), "user", "abcdefghijk", 123.5); err != nil {
		t.Fatal(err)
	}
	command := <-commands
	if command.Get("req0__sc") != "setPlaylist" || command.Get("req0_videoId") != "abcdefghijk" || command.Get("req0_currentTime") != "123.500" {
		t.Fatalf("wrong command: %v", command)
	}
	if c.runtime.handoff == nil || c.runtime.handoff.position != 123 {
		t.Fatal("missing explicit start position")
	}
}

func TestTVSendExpiredSessionRemovesPresence(t *testing.T) {
	s, _, c, _ := newTVControlHarness(t, http.StatusUnauthorized)
	c.runtime.screenOnline = true
	if err := s.SendVideo(context.Background(), "user", "abcdefghijk", 12); err == nil {
		t.Fatal("failed command was accepted")
	}
	status, err := s.PlayerStatus(context.Background(), "user")
	if err != nil || status.Online || c.runtime.handoff != nil {
		t.Fatalf("expired session remains available: %+v, %v", status, err)
	}
}

func TestTVConnectionCleanupPreservesReplacement(t *testing.T) {
	s, _, old, _ := newTVControlHarness(t, http.StatusOK)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	replacement := &tvSyncConnection{ctx: ctx, cancel: cancel, session: old.session, runtime: newTVSyncRuntime(false, nil)}
	replacement.runtime.screenOnline = true
	s.registerTVConnection("user", replacement)
	s.removeTVConnection("user", old)
	status, err := s.PlayerStatus(context.Background(), "user")
	if err != nil || !status.Online || old.ctx.Err() == nil {
		t.Fatalf("replacement status: %+v, %v", status, err)
	}
}

func TestTVHandoffDoesNotResumeOlderSavedPosition(t *testing.T) {
	h := newTVPlaybackHarness(t, 900, http.StatusOK)
	h.runtime.handoff = &tvSyncHandoff{videoID: "video", position: 12, at: time.Now()}
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 12, "state": "1"})
	h.expectProgress(t, 12)
	if len(h.commands) != 0 {
		t.Fatalf("handoff was overridden by commands: %v", h.commands)
	}
}

func TestTVHandoffSurvivesPreviousPlaybackTeardown(t *testing.T) {
	h := newTVPlaybackHarness(t, 900, http.StatusOK)
	h.runtime.handoff = &tvSyncHandoff{videoID: "video", position: 0, at: time.Now()}
	h.event(t, "nowPlaying", map[string]any{"videoId": "video"})
	h.event(t, "onStateChange", map[string]any{"videoId": "video", "currentTime": 0, "state": "0"})
	if h.runtime.handoff == nil {
		t.Fatal("previous playback consumed the handoff")
	}
	h.event(t, "nowPlaying", map[string]any{"videoId": "video", "currentTime": 0, "state": "1"})
	h.event(t, "onStateChange", map[string]any{"currentTime": 1, "state": "1"})
	h.expectProgress(t, 1)
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, command := range h.commands {
		if command == "seekTo" {
			t.Fatal("teardown discarded the explicit restart position")
		}
	}
}

func TestTVPresenceFromInitialBindAndDisconnect(t *testing.T) {
	crypto := newYouTubeSyncCrypto("secret")
	token, err := crypto.Encrypt([]byte("token"), "user")
	if err != nil {
		t.Fatal(err)
	}
	store := &mockTVSyncStore{account: &database.YouTubeTVSyncAccount{UserID: "user", ScreenID: "screen", SyncEnabled: true, LoungeTokenEnc: token, EncSecretHash: crypto.secretHash}}
	disconnect := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			chunk := `[[1,["c","sid"]],[2,["S","gsid"]],[3,["loungeStatus",{"devices":"[{\"type\":\"LOUNGE_SCREEN\"}]"}]]]`
			fmt.Fprintf(w, "%d\n%s\n", len(chunk)+1, chunk)
			return
		}
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
			return
		case <-disconnect:
		}
		chunk := `[[4,["loungeScreenDisconnected"]]]`
		fmt.Fprintf(w, "%d\n%s\n", len(chunk)+1, chunk)
	}))
	defer server.Close()
	s := &YouTubeTVSyncService{db: store, crypto: crypto, lounge: lounge.NewClientWithBaseURL(server.Client(), server.URL), noEventTimeout: time.Minute}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	account, _ := store.GetYouTubeTVSyncAccountByUserID(ctx, "user")
	go func() { done <- s.connectAndRunOnce(ctx, account) }()
	for {
		status, err := s.PlayerStatus(ctx, "user")
		if err != nil {
			t.Fatal(err)
		}
		if status.Online {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("initial receiver presence never became online")
		}
		time.Sleep(time.Millisecond)
	}
	close(disconnect)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("missing receiver disconnect error")
		}
	case <-ctx.Done():
		t.Fatal("receiver disconnect did not stop the subscription")
	}
	status, err := s.PlayerStatus(context.Background(), "user")
	if err != nil || status.Online {
		t.Fatalf("disconnected receiver still online: %+v, %v", status, err)
	}
}
