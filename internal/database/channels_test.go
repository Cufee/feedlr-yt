package database

import (
	"testing"
)

func TestGetAllChannels(t *testing.T) {
	videoLimit := 3
	c := DefaultClient
	channels, err := c.GetAllChannels(ChannelGetOptions{WithVideos: true, VideosLimit: videoLimit})
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) == 0 {
		t.Fatal("no channels")
	}
	for _, channel := range channels {
		if len(channel.Videos) > videoLimit {
			t.Fatal("too many videos")
		}
		t.Logf("TestGetAllChannels Found %v videos", len(channel.Videos))
	}
}

func TestGetAllChannelsWithSubscriptions(t *testing.T) {
	c := DefaultClient
	channels, err := c.GetAllChannelsWithSubscriptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) == 0 {
		t.Fatal("no channels")
	}
	t.Logf("TestGetAllChannelsWithSubscriptions Found %v channels", len(channels))
}
