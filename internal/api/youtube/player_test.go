package youtube

import (
	"encoding/json"
	"log"
	"testing"
)

func TestGetVideoPlayerDetails(t *testing.T) {
	client, err := NewClient("<none>", true)
	if err != nil {
		t.Error(err)
	}
	{
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
	{
		video, err := client.GetVideoPlayerDetails("LaRKIwpGPTU")
		if err != nil {
			t.Error(err)
		}

		e, err := json.MarshalIndent(video, "", "  ")
		if err != nil {
			t.Error(err)
		}

		if video.Type != VideoTypeVideo {
			t.Error("expected regular video")
		}
		if video.Duration <= 200 {
			t.Error("invalid video duration")
		}
		log.Print(string(e))
	}
	{
		video, err := client.GetVideoPlayerDetails("OSd9935ltj8")
		if err != nil {
			t.Error(err)
		}

		e, err := json.MarshalIndent(video, "", "  ")
		if err != nil {
			t.Error(err)
		}

		if video.Type != VideoTypeShort {
			t.Error("expected short video")
		}
		log.Print(string(e))
	}
}
