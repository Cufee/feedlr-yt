package types

import (
	"log"

	"github.com/byvko-dev/youtube-app/internal/api/sponsorblock"
	yt "github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/goccy/go-json"
)

type NavbarProps struct {
	CurrentURL string
	BackURL    string
	Hide       bool
}

type ChannelProps struct {
	yt.Channel
	Favorite bool
}

type ChannelSearchResultProps struct {
	yt.Channel
	Subscribed bool
}

func (c *ChannelProps) WithVideos(videos ...VideoProps) ChannelWithVideosProps {
	return ChannelWithVideosProps{
		ChannelProps: *c,
		Videos:       videos,
	}
}

type UserSubscriptionsFeedProps struct {
	Favorites        []ChannelWithVideosProps
	WithNewVideos    []ChannelWithVideosProps
	WithoutNewVideos []ChannelWithVideosProps
	All              []ChannelWithVideosProps
}

func (u *UserSubscriptionsFeedProps) ToMap() (map[string]any, error) {
	m := make(map[string]any)
	m["All"] = u.All
	m["Favorites"] = u.Favorites
	m["WithNewVideos"] = u.WithNewVideos
	m["WithoutNewVideos"] = u.WithoutNewVideos
	return m, nil
}

type ChannelWithVideosProps struct {
	ChannelProps
	Videos   []VideoProps
	CaughtUp bool
}

type VideoProps struct {
	yt.Video
	ChannelID string
	Progress  int
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

func VideoToProps(video yt.Video, channelId string) VideoProps {
	return VideoProps{
		Video:     video,
		ChannelID: channelId,
	}
}
