package types

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/goccy/go-json"
)

type SettingsPageProps struct {
	FeedMode     string
	PlayerVolume int
	SponsorBlock SponsorBlockSettingsProps
	Passkeys     []PasskeyProps
}

func (s *SettingsPageProps) Decode(record *models.Setting) error {
	return json.Unmarshal(record.Data, s)
}
func (s *SettingsPageProps) Encode() ([]byte, error) {
	return json.Marshal(s)
}

type PasskeyProps struct {
	ID        string
	Label     string
	CreatedAt time.Time
}

type SponsorBlockSettingsProps struct {
	SponsorBlockEnabled             bool
	SelectedSponsorBlockCategories  []string
	AvailableSponsorBlockCategories []sponsorblock.Category
}

type NavbarProps struct {
	CurrentURL string
	BackURL    string
	Hide       bool
}

type VideoFilter string

const (
	VideoFilterAll     VideoFilter = "all"
	VideoFilterVideos  VideoFilter = "videos"
	VideoFilterStreams VideoFilter = "streams"
)

type ChannelProps struct {
	youtube.Channel
	Favorite    bool
	VideoFilter VideoFilter
}

type ChannelSearchResultProps struct {
	youtube.Channel
	Subscribed bool
}

func (c *ChannelProps) WithVideos(videos ...VideoProps) ChannelWithVideosProps {
	return ChannelWithVideosProps{
		ChannelProps: *c,
		Videos:       videos,
	}
}

type UserVideoFeedProps struct {
	New        []VideoProps
	Watched    []VideoProps
	WatchLater []VideoProps
}

type ChannelPageProps struct {
	Authenticated bool
	Subscribed    bool
	VideoFilter   VideoFilter
	Channel       ChannelWithVideosProps
}

type ChannelWithVideosProps struct {
	ChannelProps
	Videos []VideoProps
}

type VideoProps struct {
	youtube.Video
	Progress     int
	Hidden       bool
	InWatchLater bool
	Channel      ChannelProps
	PublishedAt  time.Time
	CreatedAt    time.Time
}

type SegmentProps struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type VideoPlayerProps struct {
	Authenticated bool `json:"authenticated"`

	Video          VideoProps `json:"video"`
	ReportProgress bool       `json:"reportProgress"`

	PlayerVolumeLevel int `json:"playerVolumeLevel"`

	SkipSegments     []SegmentProps `json:"skipSegments"`
	SkipSegmentsJSON string         `json:"skipSegmentsJSON"`

	ReturnURL string `json:"returnURL"`
}

func (v *VideoPlayerProps) AddSegments(segments ...sponsorblock.Segment) error {
	for _, segment := range segments {
		if len(segment.Segment) != 2 {
			continue
		}
		v.SkipSegments = append(v.SkipSegments, SegmentProps{
			Start: int(segment.Segment[0]),
			End:   int(segment.Segment[1]),
		})
	}

	encoded, err := json.Marshal(v.SkipSegments)
	if err != nil {
		return err
	}
	v.SkipSegmentsJSON = string(encoded)
	return nil
}
