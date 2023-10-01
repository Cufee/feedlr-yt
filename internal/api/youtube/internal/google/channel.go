package google

import (
	"errors"

	yt "github.com/byvko-dev/youtube-app/internal/api/youtube/client"
)

func (c *client) SearchChannels(query string, limit int) ([]yt.Channel, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).Q(query).Type("channel").MaxResults((int64(limit))).Do()
	if err != nil {
		return nil, err
	}

	var channels []yt.Channel
	for _, item := range res.Items {
		channels = append(channels, yt.Channel{
			ID:          item.Id.ChannelId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			URL:         c.buildChannelURL(item.Id.ChannelId),
		})
	}

	return channels, nil
}

func (c *client) GetChannel(channelID string) (*yt.Channel, error) {
	res, err := c.service.Channels.List([]string{"id", "snippet"}).Id(channelID).Do()
	if err != nil {
		return nil, err
	}

	if len(res.Items) <= 0 {
		return nil, errors.New("channel not found")
	}

	var channel yt.Channel
	channel.ID = res.Items[0].Id
	channel.Title = res.Items[0].Snippet.Title
	channel.Thumbnail = res.Items[0].Snippet.Thumbnails.Default.Url

	return &channel, nil
}

func (c *client) GetChannelVideos(channelID string, limit int) ([]yt.Video, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).ChannelId(channelID).Type("video").MaxResults(int64(limit)).Do()
	if err != nil {
		return nil, err
	}

	var videos []yt.Video
	for _, item := range res.Items {
		videos = append(videos, yt.Video{
			ID:          item.Id.VideoId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			URL:         c.buildVideoEmbedURL(item.Id.VideoId),
		})
	}

	return videos, nil
}
