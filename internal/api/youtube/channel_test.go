package youtube

import (
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"
)

func TestGetChannelVideos(t *testing.T) {
	client, err := NewClient(os.Getenv("YOUTUBE_API_KEY"), nil)
	if err != nil {
		t.Error(err)
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
