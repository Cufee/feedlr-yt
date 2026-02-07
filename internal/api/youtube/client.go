package youtube

import (
	"context"
	"fmt"

	"github.com/cufee/feedlr-yt/internal/api/youtube/auth"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type client struct {
	service *youtube.Service
	auth    *auth.Client
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

func NewClient(apiKey string, auth *auth.Client) (*client, error) {
	if apiKey == "" {
		return nil, errors.New("youtube api key empty")
	}
	if auth == nil {
		return nil, errors.New("auth client is required")
	}

	c := &client{auth: auth}
	opts := option.WithAPIKey(apiKey)
	service, err := youtube.NewService(context.Background(), opts)
	metrics.ObserveYouTubeAPICall("data_v3", "new_service", err)
	if err != nil {
		return nil, err
	}
	c.service = service
	return c, nil
}
