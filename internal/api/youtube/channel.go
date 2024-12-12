package youtube

import (
	"errors"
	"time"
)

type Channel struct {
	ID          string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}
type Video struct {
	Type        VideoType
	ID          string
	URL         string
	Title       string
	Duration    int
	Thumbnail   string
	PublishedAt time.Time
	Description string
}

func (c *client) SearchChannels(query string, limit int) ([]Channel, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).Q(query).Type("channel").MaxResults((int64(limit))).Do()
	if err != nil {
		return nil, errors.Join(errors.New("SearchChannels.youtube.service.Search.List"), err)
	}

	var channels []Channel
	for _, item := range res.Items {
		channels = append(channels, Channel{
			ID:          item.Id.ChannelId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			URL:         c.BuildChannelURL(item.Id.ChannelId),
		})
	}

	return channels, nil
}

func (c *client) GetChannel(channelID string) (*Channel, error) {
	res, err := c.service.Channels.List([]string{"id", "snippet"}).Id(channelID).Do()
	if err != nil {
		return nil, errors.Join(errors.New("GetChannel.youtube.service.Channels.List"), err)
	}

	if len(res.Items) <= 0 {
		return nil, errors.New("GetChannel.youtube.service.Channels.List: no channels found")
	}

	var channel Channel
	channel.ID = res.Items[0].Id
	channel.Title = res.Items[0].Snippet.Title
	channel.Thumbnail = res.Items[0].Snippet.Thumbnails.Medium.Url
	channel.Description = res.Items[0].Snippet.Description

	return &channel, nil
}

func (c *client) GetChannelVideos(channelID string, uploadedAfter time.Time, limit int, skipVideoIds ...string) ([]Video, error) {
	uploadsId, err := c.GetChannelUploadPlaylistID(channelID)
	if err != nil {
		return nil, errors.Join(errors.New("GetChannelVideos.youtube.GetChannelUploadPlaylistID"), err)
	}

	videos, err := c.GetPlaylistVideos(uploadsId, uploadedAfter, limit, skipVideoIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetChannelVideos.youtube.GetPlaylistVideos"), err)
	}
	return videos, nil
}
