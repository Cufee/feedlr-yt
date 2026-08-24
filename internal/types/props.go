package types

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/goccy/go-json"
)

type SettingsPageProps struct {
	FeedMode      string
	PlayerVolume  int
	SponsorBlock  SponsorBlockSettingsProps
	Passkeys      []PasskeyProps
	YouTubeSync   YouTubeSyncStatusProps
	YouTubeTVSync YouTubeTVSyncStatusProps
}

type YouTubeSyncStatusProps struct {
	Available    bool
	Connected    bool
	Enabled      bool
	PlaylistID   string
	LastError    string
	LastSyncedAt time.Time
}

type YouTubeTVSyncStatusProps struct {
	Available bool
	Connected bool
	Enabled   bool

	ConnectionState string
	StateReason     string
	ScreenName      string
	LastError       string

	LastConnectedAt    time.Time
	LastEventAt        time.Time
	LastDisconnectAt   time.Time
	LastUserActivityAt time.Time
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

// MediaSource identifies where an item comes from in mixed feeds.
type MediaSource string

const (
	MediaSourceAll      MediaSource = "all"
	MediaSourceYouTube  MediaSource = "youtube"
	MediaSourcePodcasts MediaSource = "podcasts"
)

func ParseMediaSource(value string) (MediaSource, bool) {
	source := MediaSource(value)
	switch source {
	case MediaSourceAll, MediaSourceYouTube, MediaSourcePodcasts:
		return source, true
	default:
		return MediaSourceAll, false
	}
}

type ChannelProps struct {
	youtube.Channel
	Favorite      bool
	VideoFilter   VideoFilter
	FeedUpdatedAt time.Time
	IsPodcast     bool
}

func (c ChannelProps) MediaSource() MediaSource {
	if c.IsPodcast {
		return MediaSourcePodcasts
	}
	return MediaSourceYouTube
}

type ChannelSearchResultProps struct {
	youtube.Channel
	Subscribed bool
}

type PodcastSearchResultProps struct {
	FeedURL      string
	Title        string
	Description  string
	ArtworkURL   string
	Author       string
	EpisodeCount int
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
	// StreamURL is set for podcast episodes and points at the internal media stream route.
	StreamURL string
}

// IsPodcast reports whether this item is a podcast episode.
func (v VideoProps) IsPodcast() bool {
	return v.Video.Type == youtube.VideoTypePodcastEpisode
}

func (v VideoProps) MediaSource() MediaSource {
	if v.IsPodcast() {
		return MediaSourcePodcasts
	}
	return MediaSourceYouTube
}

type SegmentProps struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type PodcastSegmentProps struct {
	Category  string `json:"category"`
	StartMS   int    `json:"start_ms"`
	EndMS     int    `json:"end_ms"`
	StartText string `json:"start_text"`
	EndText   string `json:"end_text"`
	Brand     string `json:"brand"`
	Reason    string `json:"reason"`
	Skippable bool   `json:"skippable"`
}

type PodcastSegmentAnalysisProps struct {
	Status   string                `json:"status"`
	Segments []PodcastSegmentProps `json:"segments"`
}

type PlaylistProps struct {
	ID                string
	Name              string
	Description       string
	Slug              string
	VideoCount        int
	Progress          int // 0-100 percentage of videos watched
	ThumbnailVideoID  string
	YouTubePlaylistID string
	UpdatedAt         time.Time
}

type VideoPlayerProps struct {
	Authenticated bool `json:"authenticated"`

	Video          VideoProps `json:"video"`
	ReportProgress bool       `json:"reportProgress"`

	PlayerVolumeLevel int `json:"playerVolumeLevel"`

	SkipSegments     []SegmentProps              `json:"skipSegments"`
	SkipSegmentsJSON string                      `json:"skipSegmentsJSON"`
	PodcastSegments  PodcastSegmentAnalysisProps `json:"podcastSegments"`

	ReturnURL string `json:"returnURL"`

	UserPlaylists      []PlaylistProps `json:"-"`
	VideoInPlaylistIDs map[string]bool `json:"-"`
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
