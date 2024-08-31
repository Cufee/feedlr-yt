package piped

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Playlist struct {
	Name           string   `json:"name"`
	Thumbnail      string   `json:"thumbnailUrl"`
	NextPageSlug   string   `json:"nextpage"`
	RelatedStreams []Stream `json:"relatedStreams"`
}

func (pl Playlist) NextPage(ctx context.Context, client *client) (Playlist, error) {
	if pl.NextPageSlug == "" {
		return Playlist{}, errors.New("nextpage is blank")
	}

	req, err := http.NewRequest("GET", client.apiURL.JoinPath("/nextpage/playlists/"+pl.NextPageSlug).String(), nil)
	if err != nil {
		return Playlist{}, err
	}

	res, err := client.http.Do(req.WithContext(ctx))
	if err != nil {
		return Playlist{}, err
	}
	defer res.Body.Close()

	var data Playlist
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return Playlist{}, err
	}

	return data, nil
}

func (c *client) Playlist(ctx context.Context, id string) (Playlist, error) {
	req, err := http.NewRequest("GET", c.apiURL.JoinPath(fmt.Sprintf("/playlists/%s", id)).String(), nil)
	if err != nil {
		return Playlist{}, err
	}

	res, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return Playlist{}, err
	}
	defer res.Body.Close()

	var data Playlist
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return Playlist{}, err
	}

	return data, nil
}
