package invidious

import (
	"fmt"

	yt "github.com/byvko-dev/youtube-app/internal/api/youtube/client"
)

func (c *client) SearchChannels(query string, limit int) ([]yt.Channel, error) {
	if limit < 1 {
		limit = 3
	}

	var response []channel
	opts := make(map[string]string)
	opts["q"] = query
	opts["type"] = "channel"
	opts["maxResults"] = fmt.Sprintf("%v", limit)

	err := c.request("/api/v1/search", &response, opts)
	if err != nil {
		return nil, err
	}

	var channels []yt.Channel
	for i, item := range response {
		c := yt.Channel{
			ID:          item.AuthorID,
			URL:         c.buildChannelURL(item.AuthorID),
			Title:       item.Author,
			Description: item.Description,
		}
		if len(item.AuthorThumbnails) > 0 {
			c.Thumbnail = item.AuthorThumbnails[0].URL
		}
		channels = append(channels, c)
		if i >= limit-1 {
			break
		}
	}

	return channels, nil
}

func (c *client) GetChannel(channelID string) (*yt.Channel, error) {
	var res channel
	err := c.request("/api/v1/channels/"+channelID, res, nil)
	if err != nil {
		return nil, err
	}

	channel := yt.Channel{
		ID:          res.AuthorID,
		URL:         c.buildChannelURL(res.AuthorID),
		Title:       res.Author,
		Description: res.Description,
	}
	if len(res.AuthorThumbnails) > 0 {
		channel.Thumbnail = res.AuthorThumbnails[0].URL
	}

	return &channel, nil
}

func (c *client) GetChannelVideos(channelID string, limit int) ([]yt.Video, error) {
	if limit < 1 {
		limit = 3
	}

	var res struct {
		Videos []video `json:"videos"`
	}
	err := c.request(fmt.Sprintf("/api/v1/channels/%v/videos", channelID), &res, nil)
	if err != nil {
		return nil, err
	}

	var videos []yt.Video
	for i, item := range res.Videos {
		v := yt.Video{
			ID:          item.VideoID,
			URL:         c.buildVideoEmbedURL(item.VideoID),
			Title:       item.Title,
			Description: item.Description,
		}
		v.Thumbnail = c.buildVideoThumbnailURL(item.VideoID)

		videos = append(videos, v)
		if i >= limit-1 {
			break
		}
	}

	return videos, nil
}
