package youtube

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type DesktopPlayerResponse struct {
	StreamingData      StreamingData             `json:"streamingData"`
	PlayabilityStatus  DesktopPlayabilityStatus  `json:"playabilityStatus"`
	PlayerVideoDetails DesktopPlayerVideoDetails `json:"videoDetails"`
	Microformat        Microformat               `json:"microformat"`
}

type DesktopPlayabilityStatus struct {
	Status          string   `json:"status"`
	Reason          string   `json:"reason"`
	Messages        []string `json:"messages"`
	PlayableInEmbed bool     `json:"playableInEmbed"`
}

func (s DesktopPlayabilityStatus) inferPrivate() bool {
	for _, message := range s.Messages {
		if strings.Contains(message, "private video") {
			return true
		}
	}
	return false
}

type DesktopPlayerVideoDetails struct {
	VideoID           string    `json:"videoId"`
	Title             string    `json:"title"`
	LengthSeconds     string    `json:"lengthSeconds"`
	ChannelID         string    `json:"channelId"`
	IsOwnerViewing    bool      `json:"isOwnerViewing"`
	ShortDescription  string    `json:"shortDescription"`
	IsCrawlable       bool      `json:"isCrawlable"`
	Thumbnail         Thumbnail `json:"thumbnail"`
	AllowRatings      bool      `json:"allowRatings"`
	ViewCount         string    `json:"viewCount"`
	Author            string    `json:"author"`
	IsPrivate         bool      `json:"isPrivate"`
	IsUnpluggedCorpus bool      `json:"isUnpluggedCorpus"`
	IsLive            bool      `json:"isLive"`
	IsLiveContent     bool      `json:"isLiveContent"`
	IsUpcoming        bool      `json:"isUpcoming"`
}

// isShortsURL checks if YouTube serves the /shorts/ URL for this video (200 = short, 303 redirect = not)
func (c *client) isShortsURL(videoID string) bool {
	client := c.httpClientWithTimeout(5 * time.Second)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Head("https://www.youtube.com/shorts/" + videoID)
	metrics.ObserveYouTubeAPICall("player", "shorts_head_probe", err)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *client) httpClientWithTimeout(timeout time.Duration) *http.Client {
	if c.http == nil {
		return &http.Client{Timeout: timeout}
	}

	cloned := *c.http
	cloned.Timeout = timeout
	return &cloned
}

// isThumbnailPortrait checks if any thumbnail has portrait orientation (width < height)
// which indicates a Shorts video (9:16 aspect ratio, e.g., 405x720)
func isThumbnailPortrait(thumbnail Thumbnail) bool {
	for _, t := range thumbnail.Thumbnails {
		if t.Width > 0 && t.Height > 0 && t.Width < t.Height {
			return true
		}
	}
	return false
}

func (c *client) getDesktopPlayerDetails(videoId string, tries ...int) (*VideoDetails, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token, err := c.auth.Token(ctx)
	metrics.ObserveYouTubeAPICall("player", "resolve_auth_token", err)
	if err != nil {
		return nil, err
	}

	bodyContext, err := c.auth.GetContext(ctx)
	metrics.ObserveYouTubeAPICall("player", "resolve_player_context", err)
	if err != nil {
		return nil, err
	}

	body, err := bodyContext.ForVideo(token, videoId)
	metrics.ObserveYouTubeAPICall("player", "build_player_payload", err)
	if err != nil {
		return nil, err
	}

	client := c.httpClientWithTimeout(10 * time.Second)
	req, err := http.NewRequest("POST", playerURL.String(), body)
	metrics.ObserveYouTubeAPICall("player", "build_player_request", err)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	metrics.ObserveYouTubeAPICall("player", "player_request", err)
	if err != nil {
		if len(tries) > 0 && tries[0] > 0 {
			return c.getDesktopPlayerDetails(videoId, tries[0]-1)
		}
		return nil, err
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	metrics.ObserveYouTubeAPICall("player", "player_response_body", err)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		metrics.ObserveYouTubeAPICall("player", "player_status", errors.New("non_200_status"))
		if len(tries) > 0 && tries[0] > 0 {
			return c.getDesktopPlayerDetails(videoId, tries[0]-1)
		}
		log.Debug().Str("body", string(responseBody)).Int("status", res.StatusCode).Msg("invalid response")
		return nil, errors.New("bad response status code")
	}
	metrics.ObserveYouTubeAPICall("player", "player_status", nil)

	var details DesktopPlayerResponse
	err = json.Unmarshal(responseBody, &details)
	metrics.ObserveYouTubeAPICall("player", "parse_player_response", err)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse response body")
	}

	// Get duration from primary source, fallback to microformat
	duration, _ := strconv.Atoi(details.PlayerVideoDetails.LengthSeconds)
	if duration == 0 {
		duration, _ = strconv.Atoi(details.Microformat.PlayerMicroformatRenderer.LengthSeconds)
	}
	fullDetails := VideoDetails{
		ChannelID: details.PlayerVideoDetails.ChannelID,
		Duration:  duration,
		Video: Video{
			Type:        VideoTypeVideo,
			ID:          videoId,
			Title:       details.PlayerVideoDetails.Title,
			Duration:    duration,
			Description: details.PlayerVideoDetails.ShortDescription,
			Thumbnail:   c.BuildVideoThumbnailURL(videoId),
			URL:         c.BuildVideoEmbedURL(videoId),
		},
	}
	if details.Microformat.PlayerMicroformatRenderer.PublishDate != "" {
		fullDetails.Video.PublishedAt, _ = time.Parse(time.RFC3339, details.Microformat.PlayerMicroformatRenderer.PublishDate)
	}

	// Check if a video is live content (stream)
	if details.PlayerVideoDetails.IsLiveContent {
		if details.PlayerVideoDetails.IsLive {
			// Currently live
			fullDetails.Type = VideoTypeLiveStream
			return &fullDetails, nil
		} else if duration == 0 {
			// Not live yet, no duration = upcoming
			fullDetails.Type = VideoTypeUpcomingStream
			return &fullDetails, nil
		} else {
			// Has duration, not live = past stream recording
			fullDetails.Type = VideoTypeStreamRecording
			return &fullDetails, nil
		}
	}

	if details.PlayerVideoDetails.IsPrivate || details.PlayabilityStatus.inferPrivate() {
		fullDetails.Type = VideoTypePrivate
		return &fullDetails, nil
	}

	// Some other issue, not a private video explicitly
	if details.PlayabilityStatus.Status == "LOGIN_REQUIRED" {
		log.Warn().Str("video", videoId).Str("reason", details.PlayabilityStatus.Reason).Strs("messages", details.PlayabilityStatus.Messages).Msg("login required to view content")
		fullDetails.Type = VideoTypeFailed
		return &fullDetails, nil
	}

	// Check if this video is a Short and get duration if needed
	hasStreamingData := len(details.StreamingData.Formats) > 0 || len(details.StreamingData.AdaptiveFormats) > 0

	for _, format := range details.StreamingData.Formats {
		if fullDetails.Duration == 0 {
			duration, _ := strconv.Atoi(format.ApproxDurationMs)
			if !slices.Contains(invalidVideoDurations, duration) {
				fullDetails.Duration = duration / 1000
			}
		}
		if format.Width < format.Height {
			fullDetails.Type = VideoTypeShort
			return &fullDetails, nil
		}
	}
	for _, format := range details.StreamingData.AdaptiveFormats {
		if fullDetails.Duration == 0 {
			duration, _ := strconv.Atoi(format.ApproxDurationMs)
			if !slices.Contains(invalidVideoDurations, duration) {
				fullDetails.Duration = duration / 1000
			}
		}
		if format.Width < format.Height {
			fullDetails.Type = VideoTypeShort
			return &fullDetails, nil
		}
	}

	// Fallback: check thumbnail aspect ratio for Shorts (9:16, e.g., 405x720)
	if isThumbnailPortrait(details.PlayerVideoDetails.Thumbnail) {
		fullDetails.Type = VideoTypeShort
		return &fullDetails, nil
	}

	// Fallback: check if /shorts/{id} URL resolves (200 = short, 303 = not short)
	if fullDetails.Duration > 0 && fullDetails.Duration <= 180 {
		if c.isShortsURL(videoId) {
			fullDetails.Type = VideoTypeShort
			return &fullDetails, nil
		}
	}

	// Duration-based Short detection
	// Use stricter threshold (60s) when we have streaming data to verify
	// Use lenient threshold (90s) when streaming data is missing (can't check dimensions)
	shortDurationThreshold := 60
	if !hasStreamingData {
		shortDurationThreshold = 90
	}
	if fullDetails.Duration > 0 && fullDetails.Duration <= shortDurationThreshold {
		fullDetails.Type = VideoTypeShort
	}

	if !hasStreamingData && fullDetails.Duration == 0 {
		log.Warn().Str("video", videoId).Msg("missing streaming data and duration - possible rate limiting")
	}

	return &fullDetails, nil
}
