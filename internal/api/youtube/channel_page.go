package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cufee/feedlr-yt/internal/metrics"
)

var channelIDPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
var initialDataAssignment = regexp.MustCompile(`(?:var\s+ytInitialData|window\["ytInitialData"\]|ytInitialData)\s*=\s*`)

// ChannelUploadsPlaylistID derives the canonical uploads playlist for a
// channel. YouTube channel IDs use a UC prefix and their uploads playlists use
// the same suffix with a UU prefix.
func ChannelUploadsPlaylistID(channelID string) (string, error) {
	if !channelIDPattern.MatchString(channelID) {
		return "", fmt.Errorf("invalid YouTube channel ID")
	}
	return "UU" + channelID[2:], nil
}

// GetChannelPage fetches public channel metadata without credentials or Data API quota.
func (c *client) GetChannelPage(ctx context.Context, channelID string) (channel *Channel, err error) {
	defer func() { metrics.ObserveYouTubeAPICall("web", "get_channel_page", err) }()
	if !channelIDPattern.MatchString(channelID) {
		return nil, fmt.Errorf("invalid YouTube channel ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.youtube.com/channel/"+channelID+"?hl=en", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	res, err := c.httpClientWithTimeout(10 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("channel page returned HTTP %d", res.StatusCode)
	}
	const maxPageBytes = 8 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPageBytes {
		return nil, fmt.Errorf("channel page exceeds size limit")
	}
	return parseChannelPage(strings.NewReader(string(body)), channelID)
}

func parseChannelPage(page io.Reader, channelID string) (*Channel, error) {
	doc, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		return nil, err
	}
	var result *Channel
	doc.Find("script").EachWithBreak(func(_ int, script *goquery.Selection) bool {
		text := script.Text()
		match := initialDataAssignment.FindStringIndex(text)
		if match == nil {
			return true
		}
		var data struct {
			Metadata struct {
				Channel struct {
					ID          string    `json:"externalId"`
					Title       string    `json:"title"`
					Description string    `json:"description"`
					Avatar      Thumbnail `json:"avatar"`
				} `json:"channelMetadataRenderer"`
			} `json:"metadata"`
		}
		// Decode one JSON value: script content may include a trailing semicolon
		// and additional JavaScript. Never execute that JavaScript.
		if json.NewDecoder(strings.NewReader(text[match[1]:])).Decode(&data) != nil {
			return true
		}
		metadata := data.Metadata.Channel
		if metadata.ID != channelID || strings.TrimSpace(metadata.Title) == "" {
			return true
		}
		thumbnail := ""
		for _, image := range metadata.Avatar.Thumbnails {
			if image.URL != "" {
				thumbnail = image.URL
			}
		}
		if strings.HasPrefix(thumbnail, "//") {
			thumbnail = "https:" + thumbnail
		}
		result = &Channel{
			ID: metadata.ID, Title: metadata.Title, Description: metadata.Description,
			Thumbnail: thumbnail, URL: "https://www.youtube.com/channel/" + channelID,
		}
		return false
	})
	if result == nil {
		return nil, fmt.Errorf("channel page contains no matching channel metadata")
	}
	return result, nil
}
