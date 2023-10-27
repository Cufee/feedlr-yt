package database

import (
	"testing"
)

func TestGetAllChannels(t *testing.T) {
	c := DefaultClient
	channels, err := c.GetAllChannels(ChannelGetOptions{WithVideos: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) == 0 {
		t.Fatal("no channels")
	}
	t.Logf("TestGetAllChannels Found %v channels", len(channels))
	t.Logf("TestGetAllChannels First channel: %+v", channels[0])
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
