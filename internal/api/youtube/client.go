package youtube

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type client struct {
	service *youtube.Service
	auth    *OAuth2Client
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

func NewClient(apiKey string, authEnabled bool) (*client, error) {
	if apiKey == "" {
		return nil, errors.New("youtube api key empty")
	}

	c := &client{}
	opts := option.WithAPIKey(apiKey)
	service, err := youtube.NewService(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	c.service = service

	if authEnabled {
		authClient := NewOAuthClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()

		url, code, err := authClient.Authenticate(ctx)
		if err != nil {
			return nil, err
		}

		log.Info().Str("url", url).Str("code", code).Msg("Waiting for authenctication")
		status := authClient.AuthStatus()
		if status != AuthStatusAuthenticated {
			return nil, errors.Errorf("bad authentication status: %d", status)
		}
		c.auth = authClient
	}

	return c, nil
}
