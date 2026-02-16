package youtube

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/sethvargo/go-retry"
)

var (
	ErrLoginRequired = errors.New("login required")
)

type VideoType string

// Not sure why, but some videos end up with the duration values below every now and then
var invalidVideoDurations = []int{93}

const (
	VideoTypeUpcomingStream  VideoType = "upcoming_stream"
	VideoTypeLiveStream      VideoType = "live_stream"
	VideoTypeStreamRecording VideoType = "stream_recording"
	VideoTypeVideo           VideoType = "video"
	VideoTypeShort           VideoType = "short"
	VideoTypePrivate         VideoType = "private"
	VideoTypeFailed          VideoType = "failed"
)

type VideoDetails struct {
	Video
	ChannelID string `json:"channelId"`
	Duration  int    `json:"duration"`
}

type Microformat struct {
	PlayerMicroformatRenderer PlayerMicroformatRenderer `json:"playerMicroformatRenderer"`
}
type PlayerMicroformatRenderer struct {
	Thumbnail            Thumbnail            `json:"thumbnail"`
	LengthSeconds        string               `json:"lengthSeconds"`
	OwnerProfileURL      string               `json:"ownerProfileUrl"`
	ExternalChannelID    string               `json:"externalChannelId"`
	IsFamilySafe         bool                 `json:"isFamilySafe"`
	AvailableCountries   []string             `json:"availableCountries"`
	IsUnlisted           bool                 `json:"isUnlisted"`
	HasYpcMetadata       bool                 `json:"hasYpcMetadata"`
	ViewCount            string               `json:"viewCount"`
	Category             string               `json:"category"`
	PublishDate          string               `json:"publishDate"`
	OwnerChannelName     string               `json:"ownerChannelName"`
	LiveBroadcastDetails LiveBroadcastDetails `json:"liveBroadcastDetails"`
	UploadDate           string               `json:"uploadDate"`
}
type LiveBroadcastDetails struct {
	IsLiveNow    bool      `json:"isLiveNow"`
	EndTimestamp time.Time `json:"endTimestamp"`
}
type StreamingData struct {
	ExpiresInSeconds string            `json:"expiresInSeconds"`
	Formats          []Formats         `json:"formats"`
	AdaptiveFormats  []AdaptiveFormats `json:"adaptiveFormats"`
}

type Formats struct {
	Itag             int    `json:"itag"`
	URL              string `json:"url"`
	MimeType         string `json:"mimeType"`
	Bitrate          int    `json:"bitrate"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	LastModified     string `json:"lastModified"`
	ContentLength    string `json:"contentLength,omitempty"`
	Quality          string `json:"quality"`
	Fps              int    `json:"fps"`
	QualityLabel     string `json:"qualityLabel"`
	ProjectionType   string `json:"projectionType"`
	AverageBitrate   int    `json:"averageBitrate,omitempty"`
	AudioQuality     string `json:"audioQuality"`
	ApproxDurationMs string `json:"approxDurationMs"`
	AudioSampleRate  string `json:"audioSampleRate"`
	AudioChannels    int    `json:"audioChannels"`
}

type AdaptiveFormats struct {
	Itag             int     `json:"itag"`
	URL              string  `json:"url"`
	MimeType         string  `json:"mimeType"`
	Bitrate          int     `json:"bitrate"`
	Width            int     `json:"width,omitempty"`
	Height           int     `json:"height,omitempty"`
	LastModified     string  `json:"lastModified"`
	ContentLength    string  `json:"contentLength"`
	Quality          string  `json:"quality"`
	Fps              int     `json:"fps,omitempty"`
	QualityLabel     string  `json:"qualityLabel,omitempty"`
	ProjectionType   string  `json:"projectionType"`
	AverageBitrate   int     `json:"averageBitrate"`
	ApproxDurationMs string  `json:"approxDurationMs"`
	HighReplication  bool    `json:"highReplication,omitempty"`
	AudioQuality     string  `json:"audioQuality,omitempty"`
	AudioSampleRate  string  `json:"audioSampleRate,omitempty"`
	AudioChannels    int     `json:"audioChannels,omitempty"`
	LoudnessDb       float64 `json:"loudnessDb,omitempty"`
}

type Thumbnails struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type Thumbnail struct {
	Thumbnails []Thumbnails `json:"thumbnails"`
}
type PlayerVideoDetails struct {
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

var playerURL, _ = url.Parse("https://www.youtube.com/youtubei/v1/player")

var playerLimiter = time.NewTicker(time.Second / 25)

func (c *client) GetVideoPlayerDetails(videoId string) (*VideoDetails, error) {
	var result *VideoDetails
	b := retry.WithMaxRetries(3, retry.NewConstant(500*time.Millisecond))

	err := retry.Do(context.Background(), b, func(_ context.Context) error {
		<-playerLimiter.C
		details, err := c.getDesktopPlayerDetails(videoId)
		if err != nil {
			if errors.Is(err, ErrLoginRequired) {
				metrics.ObserveYouTubeAPICall("player", "bot_detection", err)
			}
			return retry.RetryableError(err)
		}
		result = details
		return nil
	})

	metrics.ObserveYouTubeAPICall("player", "get_video_player_details", err)
	return result, err
}
