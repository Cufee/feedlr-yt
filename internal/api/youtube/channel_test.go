package youtube

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/api/option"
	youtubeapi "google.golang.org/api/youtube/v3"
)

func TestGetChannelVideosDerivesUploadsPlaylistWithoutChannelLookup(t *testing.T) {
	var requestPath string
	var playlistID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		playlistID = r.URL.Query().Get("playlistId")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	service, err := youtubeapi.NewService(
		context.Background(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(server.Client()),
		option.WithoutAuthentication(),
	)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{service: service}
	if _, err := c.GetChannelVideos("UCYO_jab_esuFRV4b17AJtAw", time.Time{}, 3); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(requestPath, "channels") {
		t.Fatalf("unexpected channel lookup: %s", requestPath)
	}
	if !strings.Contains(requestPath, "playlistItems") {
		t.Fatalf("unexpected API request: %s", requestPath)
	}
	if playlistID != "UUYO_jab_esuFRV4b17AJtAw" {
		t.Fatalf("playlistId=%q", playlistID)
	}
}

func TestGetChannelVideos(t *testing.T) {
	if os.Getenv("FEEDLR_INTEGRATION_TESTS") == "" {
		t.Skip("set FEEDLR_INTEGRATION_TESTS=1 to run integration tests against live services")
	}
	if os.Getenv("YOUTUBE_API_KEY") == "" {
		t.Skip("YOUTUBE_API_KEY not set")
	}

	client, err := NewClient(os.Getenv("YOUTUBE_API_KEY"), nil)
	if err != nil {
		t.Fatal(err)
	}

	videos, err := client.GetChannelVideos("UCXuqSBlHAE6Xw-yeJA0Tunw", time.Time{}, 3)
	if err != nil {
		t.Error(err)
	}

	if len(videos) != 3 {
		t.Errorf("expected 3 videos, got %v", len(videos))
	}

	e, err := json.MarshalIndent(videos, "", "  ")
	if err != nil {
		t.Error(err)
	}

	log.Print(string(e))
}
