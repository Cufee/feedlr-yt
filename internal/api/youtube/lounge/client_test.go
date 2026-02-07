package lounge

import (
	"fmt"
	"strings"
	"testing"
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
