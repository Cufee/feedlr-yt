package youtube

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStreamRecordingDetection(t *testing.T) {
	videos := []struct {
		id           string
		description  string
		expectedType VideoType
	}{
		{"GODPh96F0M0", "past stream (VOD)", VideoTypeStreamRecording},
		{"aARsNGL-Xwc", "regular video", VideoTypeVideo},
	}

	for _, v := range videos {
		t.Run(v.description, func(t *testing.T) {
			resp, err := fetchRawPlayerResponse(v.id)
			if err != nil {
				t.Fatalf("failed to fetch: %v", err)
			}

			var detectedType VideoType
			duration := 0
			if resp.PlayerVideoDetails.LengthSeconds != "" {
				d, _ := json.Number(resp.PlayerVideoDetails.LengthSeconds).Int64()
				duration = int(d)
			}

			if resp.PlayerVideoDetails.IsLiveContent {
				if resp.PlayerVideoDetails.IsLive {
					detectedType = VideoTypeLiveStream
				} else if duration == 0 {
					detectedType = VideoTypeUpcomingStream
				} else {
					detectedType = VideoTypeStreamRecording
				}
			} else {
				detectedType = VideoTypeVideo
			}

			if detectedType != v.expectedType {
				t.Errorf("type mismatch: got %s, want %s", detectedType, v.expectedType)
			}
		})
	}
}

func fetchRawPlayerResponse(videoID string) (*DesktopPlayerResponse, error) {
	body := `{
		"videoId": "` + videoID + `",
		"context": {
			"client": {
				"clientName": "WEB",
				"clientVersion": "2.20231219.04.00"
			}
		}
	}`

	req, err := http.NewRequest("POST", "https://www.youtube.com/youtubei/v1/player", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var resp DesktopPlayerResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
