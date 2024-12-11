package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/maps"
)

var tvClientBody map[string]any
var tvClientBodyString = `{"videoId":"","context":{"client":{"clientName":"TVHTML5_SIMPLY_EMBEDDED_PLAYER","clientVersion":"2.0","clientScreen":"EMBED"}}}`

type TVPlayerResponse struct {
	StreamingData      StreamingData        `json:"streamingData"`
	PlayabilityStatus  TVPlayabilityStatus  `json:"playabilityStatus"`
	PlayerVideoDetails TVPlayerVideoDetails `json:"videoDetails"`
}

type TVPlayerVideoDetails struct {
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
	IsLiveContent     bool      `json:"isLiveContent"`
	IsLive            bool      `json:"isLive"`
}

type TVPlayabilityStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func init() {
	tvClientBody = make(map[string]any)
	err := json.Unmarshal([]byte(tvClientBodyString), &tvClientBody)
	if err != nil {
		panic(err)
	}
}

func (c *client) getTVPlayerDetails(videoId string, tries ...int) (*VideoDetails, error) {
	if c.auth == nil {
		return nil, errors.New("authentication is required for TV endpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	token, err := c.auth.Token(ctx)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{}

	var hasProxy bool
	proxy, found := getPlayerProxy()
	if found {
		transport.Proxy = http.ProxyURL(proxy.url)
		hasProxy = true
	}

	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: transport,
	}

	body := make(map[string]any)
	maps.Copy(body, tvClientBody)

	body["videoId"] = videoId
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode tv player request payload")
	}

	req, err := http.NewRequest("POST", playerURL.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		if hasProxy {
			proxy.disableFor(time.Minute * 5)
		}
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
			return nil, errors.New("tv player: invalid response")
		}
		return c.GetVideoPlayerDetails(videoId, tries[0]-1)
	}

	var details TVPlayerResponse
	err = json.Unmarshal(responseBody, &details)
	if err != nil {
		return nil, errors.Wrap(err, "tv player: failed to unmarshal response data")
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

	if details.PlayabilityStatus.Status == "UNPLAYABLE" {
		return nil, errors.New("account is likely restricted from using the TV api")
	}

	if strings.Contains(details.PlayabilityStatus.Reason, "video is private") || strings.Contains(details.PlayabilityStatus.Reason, "video is unavailable") {
		fullDetails.Type = VideoTypePrivate
		return &fullDetails, nil
	}

	// Check if a video is a live stream
	if details.PlayerVideoDetails.IsLiveContent && duration == 0 {
		if details.PlayerVideoDetails.IsLive {
			fullDetails.Type = VideoTypeLiveStream
		} else {
			fullDetails.Type = VideoTypeUpcomingStream
		}
		return &fullDetails, nil
	}

	if len(details.StreamingData.Formats) < 1 {
		// Should this mean private?
		return nil, errors.New("video missing streaming data formats")
	}
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
		if format.LastModified != "" {
			timestamp, _ := strconv.Atoi(format.LastModified)
			publishedAt := time.UnixMicro(int64(timestamp))
			if publishedAt.After(fullDetails.PublishedAt) {
				fullDetails.PublishedAt = publishedAt
			}
		}
	}

	// Last moment hard shorts check
	if fullDetails.Duration > 0 && fullDetails.Duration <= 60 {
		fullDetails.Type = VideoTypeShort
	}

	return &fullDetails, nil
}
