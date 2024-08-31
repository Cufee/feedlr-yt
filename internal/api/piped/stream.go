package piped

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Stream struct {
	videoID         string
	URL             string `json:"url"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Thumbnail       string `json:"thumbnail"`
	ChannelURL      string `json:"uploaderUrl"`
	Description     string `json:"shortDescription"`
	Duration        int    `json:"duration"`
	UploadTimestamp int64  `json:"uploaded"`
	IsShort         bool   `json:"isShort"`
}

func (stream Stream) VideoID() string {
	if stream.videoID != "" {
		return stream.videoID
	}
	stream.videoID = strings.TrimPrefix(stream.URL, "/watch?v=")
	return stream.videoID
}

func (stream Stream) PublishDate() time.Time {
	return time.UnixMilli(stream.UploadTimestamp)
}

type Video struct {
	channelID string

	ID              string `json:"id"`
	URL             string `json:"url"`
	Type            string `json:"type"`
	Title           string `json:"title"`
	Thumbnail       string `json:"thumbnailUrl"`
	ChannelURL      string `json:"uploaderUrl"`
	Description     string `json:"shortDescription"`
	Duration        int    `json:"duration"`
	UploadTimestamp int64  `json:"uploaded"`

	Visibility string `json:"visibility"`
	LiveStream bool   `json:"livestream"`
}

func (video Video) PublishDate() time.Time {
	return time.UnixMilli(video.UploadTimestamp)
}

func (video Video) ChannelID() string {
	if video.channelID != "" {
		return video.channelID
	}
	video.channelID = strings.TrimPrefix(video.ChannelURL, "/channel/")
	return video.channelID
}

func (c *Client) Video(ctx context.Context, id string) (Video, error) {
	req, err := http.NewRequest("GET", c.apiURL.JoinPath(fmt.Sprintf("/streams/%s", id)).String(), nil)
	if err != nil {
		return Video{}, err
	}

	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return Video{}, err
	}
	defer res.Body.Close()

	var data Video
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return Video{}, err
	}
	data.ID = id

	return data, nil
}
