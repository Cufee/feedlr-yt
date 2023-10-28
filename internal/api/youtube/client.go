package youtube

import (
	"context"
	"fmt"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type client struct {
	service *youtube.Service
}

func (c *client) BuildVideoThumbnailURL(videoID string) string {
	return fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID)
}

func (c *client) BuildVideoEmbedURL(videoID string) string {
	return fmt.Sprintf("https://www.youtube.com/embed/%v", videoID)
}

func (c *client) BuildChannelURL(ID string) string {
	return fmt.Sprintf("https://www.youtube.com/channel/%v", ID)
}

func NewClient(apiKey string) *client {
	if apiKey == "" {
		panic("youtube api key empty")
	}

	opts := option.WithAPIKey(apiKey)
	service, err := youtube.NewService(context.Background(), opts)
	if err != nil {
		panic(err)
	}

	return &client{
		service: service,
	}
}
