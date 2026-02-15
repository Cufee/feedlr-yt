package youtube

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVideoTypeDetection(t *testing.T) {
	videos := []struct {
		id           string
		description  string
		expectedType VideoType
	}{
		{"GODPh96F0M0", "past stream (VOD)", VideoTypeStreamRecording},
		{"aARsNGL-Xwc", "regular video", VideoTypeVideo},
		{"KeLmi62DmjU", "short", VideoTypeShort},
		{"H4iECWYllc4", "short with landscape thumbnail", VideoTypeShort},
		{"1GA8AngP758", "recent regression", VideoTypeShort},
	}

	for _, v := range videos {
		t.Run(v.description, func(t *testing.T) {
			resp, err := fetchRawPlayerResponse(v.id)
			if err != nil {
				t.Fatalf("failed to fetch: %v", err)
			}

			duration := 0
			if resp.PlayerVideoDetails.LengthSeconds != "" {
				d, _ := json.Number(resp.PlayerVideoDetails.LengthSeconds).Int64()
				duration = int(d)
			}

			// Log debug info
			t.Logf("Duration: %d seconds", duration)
			t.Logf("IsLiveContent: %v", resp.PlayerVideoDetails.IsLiveContent)
			t.Logf("Thumbnail portrait: %v", isThumbnailPortrait(resp.PlayerVideoDetails.Thumbnail))
			t.Logf("Formats count: %d, AdaptiveFormats count: %d", len(resp.StreamingData.Formats), len(resp.StreamingData.AdaptiveFormats))

			// Check all formats for portrait
			isPortraitFormat := false
			for _, f := range resp.StreamingData.Formats {
				if f.Width > 0 && f.Height > 0 {
					t.Logf("Format: %dx%d (portrait=%v)", f.Width, f.Height, f.Width < f.Height)
					if f.Width < f.Height {
						isPortraitFormat = true
					}
				}
			}
			for _, f := range resp.StreamingData.AdaptiveFormats {
				if f.Width > 0 && f.Height > 0 {
					if f.Width < f.Height {
						isPortraitFormat = true
						t.Logf("Adaptive portrait: %dx%d", f.Width, f.Height)
						break
					}
				}
			}
			t.Logf("Has portrait format: %v", isPortraitFormat)

			// Detection logic matching player_desktop.go
			var detectedType VideoType = VideoTypeVideo

			if resp.PlayerVideoDetails.IsLiveContent {
				if resp.PlayerVideoDetails.IsLive {
					detectedType = VideoTypeLiveStream
				} else if duration == 0 {
					detectedType = VideoTypeUpcomingStream
				} else {
					detectedType = VideoTypeStreamRecording
				}
			} else {
				// Check format dimensions for shorts
				for _, f := range resp.StreamingData.Formats {
					if f.Width < f.Height {
						detectedType = VideoTypeShort
						break
					}
				}
				if detectedType != VideoTypeShort {
					for _, f := range resp.StreamingData.AdaptiveFormats {
						if f.Width < f.Height {
							detectedType = VideoTypeShort
							break
						}
					}
				}
				// Fallback: thumbnail portrait
				if detectedType != VideoTypeShort && isThumbnailPortrait(resp.PlayerVideoDetails.Thumbnail) {
					detectedType = VideoTypeShort
				}
				// Fallback: /shorts/ URL check
				if detectedType != VideoTypeShort && duration > 0 && duration <= 180 {
					isShort := (&client{}).isShortsURL(v.id)
					t.Logf("Shorts URL check: %v", isShort)
					if isShort {
						detectedType = VideoTypeShort
					}
				}
				// Fallback: duration threshold (90s without streaming data, 60s with)
				hasStreamingData := len(resp.StreamingData.Formats) > 0 || len(resp.StreamingData.AdaptiveFormats) > 0
				threshold := 60
				if !hasStreamingData {
					threshold = 90
				}
				if detectedType != VideoTypeShort && duration > 0 && duration <= threshold {
					detectedType = VideoTypeShort
				}
			}

			t.Logf("Detected: %s, Expected: %s", detectedType, v.expectedType)

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
