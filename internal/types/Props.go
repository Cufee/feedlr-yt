package types

import (
	yt "github.com/byvko-dev/youtube-app/internal/api/youtube/client"
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

func VideoToProps(video yt.Video, channelId string) VideoProps {
	return VideoProps{
		Video:     video,
		ChannelID: channelId,
	}
}
