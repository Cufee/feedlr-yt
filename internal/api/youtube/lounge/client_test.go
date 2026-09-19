package lounge

import (
	"context"
	"fmt"
	"io"
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
	go func() { done <- client.SeekTo(ctx, session, 1200) }()
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
		t.Error("poll overtook in-flight seek")
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
