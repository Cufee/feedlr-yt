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
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	PlayableInEmbed bool   `json:"playableInEmbed"`
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

func (c *client) getDesktopPlayerDetails(videoId string, tries ...int) (*VideoDetails, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token, err := c.auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	bodyContext, err := c.auth.GetContext(ctx)
	if err != nil {
		return nil, err
	}

	body, err := bodyContext.ForVideo(token, videoId)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequest("POST", playerURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		if len(tries) < 1 || tries[0] < 2 {
			return nil, err
		}
		return c.GetVideoPlayerDetails(videoId, tries[0]-1)
	}

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		if len(tries) < 1 || tries[0] < 2 {
			log.Debug().Str("body", string(responseBody)).Int("status", res.StatusCode).Msg("invalid response")
			return nil, errors.New("GetVideoPlayerDetails.http.Post: invalid response")
		}
		return c.GetVideoPlayerDetails(videoId, tries[0]-1)
	}

	var details DesktopPlayerResponse
	err = json.Unmarshal(responseBody, &details)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse response body")
	}

	duration, _ := strconv.Atoi(details.PlayerVideoDetails.LengthSeconds)
	fullDetails := VideoDetails{
		ChannelID: details.PlayerVideoDetails.ChannelID,
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

	// Check if a video is a live stream
	// Status will not be OK if a video is an upcoming stream
	if details.PlayerVideoDetails.IsLiveContent && duration == 0 {
		if details.PlayerVideoDetails.IsLive {
			fullDetails.Type = VideoTypeLiveStream
		} else {
			fullDetails.Type = VideoTypeUpcomingStream
		}
		return &fullDetails, nil
	}

	if details.PlayerVideoDetails.IsPrivate || strings.Contains(details.PlayabilityStatus.Reason, "This is a private video") {
		fullDetails.Type = VideoTypePrivate
		return &fullDetails, nil
	}

	// Some other issue, not a private video explicitly
	if details.PlayabilityStatus.Status == "LOGIN_REQUIRED" {
		log.Warn().Str("video", videoId).Str("message", details.PlayabilityStatus.Reason).Msg("login required to view content")
		fullDetails.Type = VideoTypeFailed
		return &fullDetails, nil
	}

	// Check if this video is a Short and get duration if needed
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

	if fullDetails.Duration > 0 && fullDetails.Duration <= 60 {
		fullDetails.Type = VideoTypeShort
	}

	return &fullDetails, nil
}
