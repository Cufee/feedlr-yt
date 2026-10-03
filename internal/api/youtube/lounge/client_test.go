package lounge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseEventChunks(t *testing.T) {
	chunkOne := `[[1,["c","sid-123"]],[2,["S","gs-123"]]]`
	chunkTwo := `[[3,["nowPlaying",{"videoId":"abc","currentTime":"12.3","duration":"99","state":"1"}]]]`
	payload := fmt.Sprintf("%d\n%s\n%d\n%s\n", len(chunkOne)+1, chunkOne, len(chunkTwo)+1, chunkTwo)

	var parsed []Event
	err := parseEventChunks(strings.NewReader(payload), func(events []Event) error {
		parsed = append(parsed, events...)
		return nil
	})
	if err != nil {
		t.Fatalf("parseEventChunks returned error: %v", err)
	}

	if len(parsed) != 3 {
		t.Fatalf("expected 3 events, got %d", len(parsed))
	}
	if parsed[0].Type != "c" || parsed[1].Type != "S" || parsed[2].Type != "nowPlaying" {
		t.Fatalf("unexpected event sequence: %#v", parsed)
	}
}

func TestPlaybackSnapshotIdentityAndInvalidTimes(t *testing.T) {
	for _, value := range []any{"NaN", "+Inf", "-Inf", "-1", -1.0, nil, "bad"} {
		p, ok := ExtractPlaybackEvent(Event{Type: "nowPlaying", Args: []any{map[string]any{"videoId": "video", "currentTime": value}}})
		if !ok || p.HasCurrentTime {
			t.Errorf("invalid currentTime %v was accepted: %+v", value, p)
		}
	}
	for _, kind := range []string{"nowPlaying", "onStateChange"} {
		p, ok := ExtractPlaybackEvent(Event{Type: kind, Args: []any{map[string]any{"videoId": ""}}})
		if !ok || !p.HasVideoID || p.VideoID != "" {
			t.Errorf("lost explicit empty video ID: %+v, %v", p, ok)
		}
	}
	p, ok := ExtractPlaybackEvent(Event{Type: "onStateChange", Args: []any{map[string]any{"currentTime": "0", "state": "2"}}})
	if !ok || p.HasVideoID || !p.HasCurrentTime {
		t.Fatalf("lost partial state event: %+v", p)
	}
	if _, ok := ExtractPlaybackEvent(Event{Type: "nowPlaying", Args: []any{map[string]any{}}}); !ok {
		t.Fatal("empty snapshot must clear playback")
	}
}

type loungeTestTransport func(*http.Request) (*http.Response, error)

func (f loungeTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCommandsSerializeDispatchWithoutBlockingEvents(t *testing.T) {
	t.Run("seek", func(t *testing.T) {
		testCommandsSerializeDispatchWithoutBlockingEvents(t, func(client *Client, ctx context.Context, session *Session) error {
			return client.SeekTo(ctx, session, 1200)
		})
	})
	t.Run("play video", func(t *testing.T) {
		testCommandsSerializeDispatchWithoutBlockingEvents(t, func(client *Client, ctx context.Context, session *Session) error {
			return client.PlayVideo(ctx, session, "video", 1200)
		})
	})
}

func testCommandsSerializeDispatchWithoutBlockingEvents(t *testing.T, command func(*Client, context.Context, *Session) error) {
	t.Helper()
	firstStarted, secondStarted, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var count atomic.Int32
	session := &Session{SID: "sid", GSessionID: "gsid", commandOffset: 1}
	client := NewClient(&http.Client{Transport: loungeTestTransport(func(r *http.Request) (*http.Response, error) {
		n := count.Add(1)
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		if r.URL.Query().Get("RID") != fmt.Sprint(n+1) || form.Get("ofs") != fmt.Sprint(n) {
			t.Errorf("out of order RID/offset: %s, %s", r.URL.RawQuery, body)
		}
		if n == 1 {
			close(firstStarted)
			<-release
		} else {
			close(secondStarted)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- command(client, ctx, session) }()
	select {
	case <-firstStarted:
	case <-ctx.Done():
		t.Fatal("first command did not start")
	}
	go func() { done <- client.GetNowPlaying(ctx, session) }()
	// Stream processing must remain possible while the HTTP command is in flight.
	eventsDone := make(chan struct{})
	go func() { session.applyEvents([]Event{{ID: 42, Type: "noop"}}); close(eventsDone) }()
	select {
	case <-eventsDone:
	case <-ctx.Done():
		close(release)
		t.Fatal("command blocked event state")
	}
	select {
	case <-secondStarted:
		t.Error("poll overtook in-flight command")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("commands did not complete")
		}
	}
}

func TestRequestTimeoutCapsLongParentDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	ctx, cancelRequest := withRequestTimeout(parent, 20*time.Millisecond)
	defer cancelRequest()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 20*time.Millisecond {
		t.Fatalf("request inherited long parent deadline: %v", deadline)
	}
	shortParent, cancelShort := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelShort()
	short, cancelRequest := withRequestTimeout(shortParent, time.Hour)
	defer cancelRequest()
	want, _ := shortParent.Deadline()
	got, _ := short.Deadline()
	if !got.Equal(want) {
		t.Fatalf("request extended earlier parent deadline: %v", got)
	}
}

func TestExtractPlaybackEvent(t *testing.T) {
	event := Event{
		ID:   10,
		Type: "nowPlaying",
		Args: []any{map[string]any{
			"videoId":     "video-1",
			"currentTime": "10.5",
			"duration":    "120",
			"state":       "1",
		}},
	}

	playback, ok := ExtractPlaybackEvent(event)
	if !ok {
		t.Fatal("expected playback event")
	}
	if playback.VideoID != "video-1" {
		t.Fatalf("unexpected video id: %s", playback.VideoID)
	}
	if !playback.HasCurrentTime || playback.CurrentTime != 10.5 {
		t.Fatalf("unexpected current time: %+v", playback)
	}
	if !playback.HasDuration || playback.Duration != 120 {
		t.Fatalf("unexpected duration: %+v", playback)
	}
	if playback.State != "1" {
		t.Fatalf("unexpected state: %s", playback.State)
	}
}

func TestExtractScreenPresence(t *testing.T) {
	status := func(devices any) Event {
		return Event{Type: "loungeStatus", Args: []any{map[string]any{"devices": devices}}}
	}
	for _, tt := range []struct {
		name   string
		event  Event
		online bool
		known  bool
	}{
		{"cloud session", Event{Type: "c", Args: []any{"sid"}}, false, false},
		{"playback", Event{Type: "nowPlaying", Args: []any{map[string]any{"videoId": "video"}}}, false, false},
		{"receiver", status(`[{"type":"REMOTE_CONTROL"},{"type":"LOUNGE_SCREEN"}]`), true, true},
		{"remote only", status(`[{"type":"REMOTE_CONTROL"}]`), false, true},
		{"empty devices", status(`[]`), false, true},
		{"direct array", status([]any{map[string]any{"type": "LOUNGE_SCREEN"}}), true, true},
		{"disconnected", Event{Type: "loungeScreenDisconnected"}, false, true},
		{"missing args", Event{Type: "loungeStatus"}, false, true},
		{"invalid args", Event{Type: "loungeStatus", Args: []any{"invalid"}}, false, true},
		{"missing devices", Event{Type: "loungeStatus", Args: []any{map[string]any{}}}, false, true},
		{"malformed JSON", status(`[{"type":"LOUNGE_SCREEN"}`), false, true},
		{"null devices", status(`null`), false, true},
		{"invalid devices", status(42), false, true},
		{"object instead of array", status(`{"type":"LOUNGE_SCREEN"}`), false, true},
		{"invalid device", status(`[null]`), false, true},
		{"missing device type", status(`[{}]`), false, true},
		{"invalid device type", status(`[{"type":42}]`), false, true},
		{"malformed entry after receiver", status(`[{"type":"LOUNGE_SCREEN"},42]`), false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			online, known := ExtractScreenPresence(tt.event)
			if online != tt.online || known != tt.known {
				t.Fatalf("presence = (%v, %v), want (%v, %v)", online, known, tt.online, tt.known)
			}
		})
	}
}

func TestPlayVideoPayload(t *testing.T) {
	for _, position := range []float64{0, 123.456} {
		t.Run(fmt.Sprint(position), func(t *testing.T) {
			session := &Session{SID: "sid", GSessionID: "gsid", commandOffset: 1}
			called := false
			client := NewClient(&http.Client{Transport: loungeTestTransport(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.Method != http.MethodPost || r.URL.Path != "/api/lounge/bc/bind" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]string{
					"req0__sc":         "setPlaylist",
					"req0_videoId":     "video-1",
					"req0_currentTime": fmt.Sprintf("%.3f", position),
					"count":            "1",
					"ofs":              "1",
				} {
					if got := r.PostForm.Get(key); got != want {
						t.Errorf("%s = %q, want %q", key, got, want)
					}
				}
				if r.URL.Query().Get("RID") != "2" || r.URL.Query().Get("SID") != "sid" {
					t.Error("missing command session identifiers")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})})
			if err := client.PlayVideo(context.Background(), session, " video-1 ", position); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("video command was not sent")
			}
		})
	}
}

func TestPlayVideoRejectsInvalidInput(t *testing.T) {
	client := NewClient(&http.Client{Transport: loungeTestTransport(func(r *http.Request) (*http.Response, error) {
		t.Fatal("invalid video command reached transport")
		return nil, errors.New("unexpected request")
	})})
	session := &Session{SID: "sid", GSessionID: "gsid", commandOffset: 1}
	for _, tt := range []struct {
		name     string
		videoID  string
		position float64
	}{
		{"empty video", "", 0},
		{"blank video", " \t\n", 0},
		{"negative position", "video", -1},
		{"NaN", "video", math.NaN()},
		{"positive infinity", "video", math.Inf(1)},
		{"negative infinity", "video", math.Inf(-1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := client.PlayVideo(context.Background(), session, tt.videoID, tt.position); err == nil {
				t.Fatal("invalid command was accepted")
			}
		})
	}
	if session.commandOffset != 1 {
		t.Fatal("invalid commands consumed command IDs")
	}
	if err := client.PlayVideo(context.Background(), nil, "video", 0); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("nil session error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.PlayVideo(canceled, session, "video", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled command error = %v", err)
	}
}

func TestPlayVideoPropagatesCommandErrors(t *testing.T) {
	transportError := errors.New("transport failed")
	for _, tt := range []struct {
		name   string
		status int
		body   string
		err    error
		want   error
	}{
		{"expired auth", http.StatusUnauthorized, "Expired", nil, ErrAuthExpired},
		{"unknown session", http.StatusBadRequest, "Unknown SID", nil, ErrUnknownSID},
		{"gone session", http.StatusGone, "Gone", nil, ErrSessionGone},
		{"transport error", 0, "", transportError, transportError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(&http.Client{Transport: loungeTestTransport(func(r *http.Request) (*http.Response, error) {
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})})
			session := &Session{SID: "sid", GSessionID: "gsid", commandOffset: 1}
			if err := client.PlayVideo(context.Background(), session, "video", 0); !errors.Is(err, tt.want) {
				t.Fatalf("command error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPlayVideoDoesNotSucceedAfterResponseBodyFailure(t *testing.T) {
	bodyError := errors.New("response body interrupted")
	for _, tt := range []struct {
		name    string
		cancel  bool
		readErr error
		want    error
	}{
		{"body read error", false, bodyError, bodyError},
		{"canceled while reading body", true, context.Canceled, context.Canceled},
		{"canceled with clean EOF", true, nil, context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			bodyRead, release := make(chan struct{}), make(chan struct{})
			defer close(release)
			go func() {
				// A pipe write completes only after the command starts reading the
				// HTTP 200 response body, so cancellation cannot happen in Do.
				if _, err := writer.Write([]byte("partial response")); err != nil {
					return
				}
				close(bodyRead)
				select {
				case <-release:
				case <-ctx.Done():
				}
				writer.CloseWithError(tt.readErr)
			}()
			client := NewClient(&http.Client{Transport: loungeTestTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: reader, Header: make(http.Header)}, nil
			})})
			session := &Session{SID: "sid", GSessionID: "gsid", commandOffset: 1}
			done := make(chan error, 1)
			go func() { done <- client.PlayVideo(ctx, session, "video", 0) }()
			select {
			case <-bodyRead:
			case <-time.After(time.Second):
				t.Fatal("command did not read the response body")
			}
			if tt.cancel {
				cancel()
			} else {
				writer.CloseWithError(tt.readErr)
			}
			select {
			case err := <-done:
				if !errors.Is(err, tt.want) {
					t.Fatalf("command error = %v, want %v", err, tt.want)
				}
			case <-time.After(time.Second):
				t.Fatal("command did not finish after body failure")
			}
		})
	}
}
