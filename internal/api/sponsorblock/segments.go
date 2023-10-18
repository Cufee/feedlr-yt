package sponsorblock

import (
	"encoding/json"
	"fmt"
	"net/http"
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
func (c *client) GetVideoSegments(videoId string) ([]Segment, error) {
	var segments []Segment

	res, err := http.DefaultClient.Get(fmt.Sprintf("%s/skipSegments/?videoID=%s", c.apiUrl, videoId))
	if err != nil {
		return nil, err
	}
	if res == nil || res.StatusCode == http.StatusNotFound {
		return segments, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: unexpected status code %d", res.StatusCode)
	}

	err = json.NewDecoder(res.Body).Decode(&segments)
	if err != nil {
		return nil, err
	}

	return segments, nil
}
