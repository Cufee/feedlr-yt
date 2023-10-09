package types

import yt "github.com/byvko-dev/youtube-app/internal/api/youtube/client"

type NavbarProps struct {
	CurrentURL string
	BackURL    string
	Hide       bool
}

type ChannelProps struct {
	yt.Channel
	Favorite bool
}

func (c *ChannelProps) WithVideos(videos ...VideoProps) ChannelWithVideosProps {
	return ChannelWithVideosProps{
		ChannelProps: *c,
		Videos:       videos,
	}
}

type ChannelWithVideosProps struct {
	ChannelProps
	Videos []VideoProps
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
