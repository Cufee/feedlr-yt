package youtube

import (
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"
)

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
