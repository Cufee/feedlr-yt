package piped

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cufee/feedlr-yt/internal/logic"
)

type Stream struct {
	URL         string `json:"url"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Thumbnail   string `json:"thumbnail"`
	ChannelURL  string `json:"uploaderUrl"`
	Description string `json:"shortDescription"`
	Duration    int    `json:"uploaderAvatar"`
	PublishedAt int64  `json:"uploaded"`
	IsShort     bool   `json:"isShort"`
}

type Video struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Thumbnail   string `json:"thumbnailUrl"`
	ChannelURL  string `json:"uploaderUrl"`
	Description string `json:"shortDescription"`
	Duration    int    `json:"uploaderAvatar"`
	PublishedAt int64  `json:"uploaded"`

	Visibility string `json:"visibility"`
	LiveStream bool   `json:"livestream"`
}

func (stream Stream) VideoID() string {
	id, _ := logic.VideoIDFromURL(stream.URL)
	return id
}

func (stream Stream) PublishDate() time.Time {
	return time.UnixMilli(stream.PublishedAt)
}

func (c *client) Video(ctx context.Context, id string) (Video, error) {
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
