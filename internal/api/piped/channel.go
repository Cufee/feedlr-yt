package piped

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Channel struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Avatar         string   `json:"avatarUrl"`
	Banner         string   `json:"bannerUrl"`
	Description    string   `json:"description"`
	NextPageSlug   string   `json:"nextpage"`
	RelatedStreams []Stream `json:"relatedStreams"`
	Verified       bool     `json:"verified"`
}

func (ch Channel) NextPage(ctx context.Context, client *client) (Channel, error) {
	if ch.NextPageSlug == "" {
		return Channel{}, errors.New("nextpage is blank")
	}

	req, err := http.NewRequest("GET", client.apiURL.JoinPath("/nextpage/channel/"+ch.NextPageSlug).String(), nil)
	if err != nil {
		return Channel{}, err
	}

	res, err := client.http.Do(req.WithContext(ctx))
	if err != nil {
		return Channel{}, err
	}
	defer res.Body.Close()

	var data Channel
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return Channel{}, err
	}

	return data, nil
}

func (c *client) Channel(ctx context.Context, id string) (Channel, error) {
	req, err := http.NewRequest("GET", c.apiURL.JoinPath(fmt.Sprintf("/channel/%s", id)).String(), nil)
	if err != nil {
		return Channel{}, err
	}

	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return Channel{}, err
	}
	defer res.Body.Close()

	var data Channel
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return Channel{}, err
	}

	return data, nil
}
