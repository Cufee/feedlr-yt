package client

type YouTube interface {
	SearchChannels(query string, maxResults int) ([]Channel, error)

	GetChannel(channelId string) (*Channel, error)
	GetChannelVideos(channelId string, maxResults int, excludeIds ...string) ([]Video, error)
}

type Channel struct {
	ID          string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}

type Video struct {
	ID          string
	URL         string
	Title       string
	Duration    int
	Thumbnail   string
	PublishedAt string
	Description string
}
