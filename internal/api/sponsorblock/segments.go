package sponsorblock

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

type Segment struct {
	Segment       []float64 `json:"segment"`
	UUID          string    `json:"UUID"`
	Category      string    `json:"category"`
	VideoDuration float64   `json:"videoDuration"`
	ActionType    string    `json:"actionType"`
	Locked        int       `json:"locked"`
	Votes         int       `json:"votes"`
	Description   string    `json:"description"`
}

/*
GetVideoSegments returns a list of _sponsor_ segments for a given video ID.

https://wiki.sponsor.ajay.app/w/API_Docs#GET_/api/skipSegments
*/
func (c *client) GetVideoSegments(videoId string, categories ...Category) ([]Segment, error) {
	if len(categories) == 0 {
		categories = []Category{Sponsor, SelfPromo, Interaction}
	}

	link, err := url.Parse(fmt.Sprintf("%s/skipSegments/", c.apiUrl))
	if err != nil {
		return nil, errors.Join(errors.New("GetVideoSegments.url.Parse"), err)
	}
	query := link.Query()
	query.Add("videoID", videoId)
	for _, category := range categories {
		query.Add("category", category.Value)
	}
	link.RawQuery = query.Encode()

	var segments []Segment
	res, err := c.http.Get(link.String())
	if err != nil {
		if os.IsTimeout(err) {
			return nil, ErrRequestTimeout
		}
		return nil, errors.Join(errors.New("GetVideoSegments.http.DefaultClient.Get"), err)
	}
	if res == nil || res.StatusCode == http.StatusNotFound {
		return segments, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: unexpected status code %d", res.StatusCode)
	}

	err = json.NewDecoder(res.Body).Decode(&segments)
	if err != nil {
		return nil, errors.Join(errors.New("GetVideoSegments.json.NewDecoder.Decode"), err)
	}

	return segments, nil
}
