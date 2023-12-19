package youtube

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/cufee/feedlr-yt/internal/utils"
)

func TestGetVideoPlayerDetails(t *testing.T) {
	client := NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
	video, err := client.GetVideoPlayerDetails("JpW1KrK6Xjk")
	if err != nil {
		t.Error(err)
	}

	e, err := json.MarshalIndent(video, "", "  ")
	if err != nil {
		t.Error(err)
	}

	if video.Type != VideoTypePrivate {
		t.Error("expected private video")
	}
	log.Print(string(e))
}
