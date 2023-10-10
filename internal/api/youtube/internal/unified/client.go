package unified

import (
	"os"

	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/google"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/invidious"
)

var (
	ytClient  = google.NewClient(os.Getenv("YOUTUBE_API_KEY"))
	invClient = invidious.NewClient(os.Getenv("INVIDIOUS_HOST"))
)

type unified struct{}

func (c *unified) SearchChannels(query string, maxResults int) ([]client.Channel, error) {
	return ytClient.SearchChannels(query, maxResults)
}

func (c *unified) GetChannel(channelId string) (*client.Channel, error) {
	return ytClient.GetChannel(channelId)
}

func (c *unified) GetChannelVideos(channelId string, maxResults int) ([]client.Video, error) {
	return invClient.GetChannelVideos(channelId, maxResults)
}

func NewClient() *unified {
	return &unified{}
}
