package types

import (
	"log"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/goccy/go-json"
)

type SettingsPageProps struct {
	FeedMore     string
	SponsorBlock SponsorBlockSettingsProps
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

type ChannelProps struct {
	youtube.Channel
	Favorite bool
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

type UserSubscriptionsFeedProps struct {
	NextUpdate       int64
	Favorites        []ChannelWithVideosProps
	WithNewVideos    []ChannelWithVideosProps
	WithoutNewVideos []ChannelWithVideosProps
	All              []ChannelWithVideosProps
}

type UserVideoFeedProps struct {
	NextUpdate int64
	Videos     []VideoWithChannelProps
}

type ChannelWithVideosProps struct {
	ChannelProps
	Videos   []VideoProps
	CaughtUp bool
}

type VideoProps struct {
	youtube.Video
	ChannelID string
	Progress  int
}

type VideoWithChannelProps struct {
	VideoProps
	ChannelID        string
	ChannelTitle     string
	ChannelThumbnail string
}

type SegmentProps struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type VideoPlayerProps struct {
	Video          VideoProps `json:"video"`
	ReportProgress bool       `json:"reportProgress"`

	SkipSegments     []SegmentProps `json:"skipSegments"`
	SkipSegmentsJSON string         `json:"skipSegmentsJSON"`
}

func (v *VideoPlayerProps) AddSegments(segments ...sponsorblock.Segment) error {
	for _, segment := range segments {
		if len(segment.Segment) != 2 {
			log.Printf("segment %v for video %v has invalid length", segment, v.Video.ID)
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

func VideoToProps(video youtube.Video, channelId string) VideoProps {
	return VideoProps{
		Video:     video,
		ChannelID: channelId,
	}
}

func VideoModelToProps(video *models.Video) VideoProps {
	return VideoProps{
		Video: youtube.Video{
			ID:          video.ExternalID,
			URL:         video.URL,
			Title:       video.Title,
			Duration:    video.Duration,
			Thumbnail:   video.Thumbnail,
			PublishedAt: video.PublishedAt.String(),
			Description: video.Description,
		},
		ChannelID: video.ChannelId,
	}
}
