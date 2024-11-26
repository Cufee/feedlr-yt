package youtube

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)

func TestGetPlaylistVideos(t *testing.T) {
	client, err := NewClient(os.Getenv("YOUTUBE_API_KEY"), false)
	if err != nil {
		t.Error(err)
	}

	playlist, err := client.GetChannelUploadPlaylistID("UCUyeluBRhGPCW4rPe_UvBZQ")
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
