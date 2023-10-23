package google

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/cufee/feedlr-yt/internal/utils"
)

func TestGetChannelVideos(t *testing.T) {
	client := NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
	videos, err := client.GetChannelVideos("UCBJycsmduvYEL83R_U4JriQ", 3)
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
