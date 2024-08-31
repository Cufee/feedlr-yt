package piped

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

type SearchItem struct {
	id          string
	URL         string `json:"url"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	Verified    bool   `json:"verified"`
}

func (item SearchItem) ChannelID() string {
	if item.id != "" {
		return item.id
	}
	item.id = strings.TrimPrefix(item.URL, "/channel/")
	return item.id
}

func (c *Client) SearchChannels(ctx context.Context, query string) ([]SearchItem, error) {
	q := make(url.Values)
	q.Add("filter", "channels")
	q.Add("q", query)

	req, err := http.NewRequest("GET", c.apiURL.JoinPath("search").String()+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var data struct {
		Items []SearchItem `json:"items"`
	}
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	return data.Items, nil
}
