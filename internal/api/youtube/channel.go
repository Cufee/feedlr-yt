package youtube

import (
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/friendsofgo/errors"
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

func (v Video) isShort() bool {
	if v.Type == VideoTypeShort {
		return true
	}
	if v.Type != VideoTypeVideo || v.Duration <= 0 {
		return false
	}
	if v.Duration <= 60 {
		return true
	}
	// Shorts can be longer now; use explicit metadata markers to avoid false positives.
	return v.Duration <= 180 && looksLikeShortsMetadata(v.Title, v.Description)
}

func looksLikeShortsMetadata(values ...string) bool {
	for _, value := range values {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "#shorts") || strings.Contains(lower, "#short") || strings.Contains(lower, "/shorts/") {
			return true
		}
	}
	return false
}

func (c *client) SearchChannels(query string, limit int) ([]Channel, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).Q(query).Type("channel").MaxResults((int64(limit))).Do()
	metrics.ObserveYouTubeAPICall("data_v3", "search_channels", err)
	if err != nil {
		return nil, errors.Wrap(err, "search failed")
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
	metrics.ObserveYouTubeAPICall("data_v3", "get_channel", err)
	if err != nil {
		return nil, errors.Wrap(err, "channels list failed")
	}

	if len(res.Items) <= 0 {
		return nil, errors.New("channels list returned no channels")
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
	metrics.ObserveYouTubeAPICall("data_v3", "get_channel_videos_upload_playlist", err)
	if err != nil {
		return nil, err
	}

	videos, err := c.GetPlaylistVideos(uploadsId, uploadedAfter, limit, skipVideoIds...)
	metrics.ObserveYouTubeAPICall("data_v3", "get_channel_videos_playlist", err)
	if err != nil {
		return nil, err
	}
	return videos, nil
}
