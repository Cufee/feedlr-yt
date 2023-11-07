package youtube

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/cufee/feedlr-yt/internal/utils"
)

func TestGetPlaylistVideos(t *testing.T) {
	client := NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
	playlist, err := client.GetChannelUploadPlaylistID("UCQSG4J_ssXdZXI4b36RpkRQ")
	if err != nil {
		t.Error(err)
	}

	videos, err := client.GetPlaylistVideos(playlist, 3)
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
