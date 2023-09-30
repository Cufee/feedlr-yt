package types

import "fmt"

type NavbarProps struct {
	CurrentURL string
	BackURL    string
	Hide       bool
}

type ChannelProps struct {
	ID          string
	Title       string
	Thumbnail   string
	Description string
	Favorite    bool
}

func (c *ChannelProps) WithVideos(videos ...VideoProps) *ChannelWithVideosProps {
	return &ChannelWithVideosProps{
		ChannelProps: c,
		Videos:       videos,
	}
}

type ChannelWithVideosProps struct {
	*ChannelProps
	Videos []VideoProps
}

type VideoProps struct {
	ID          string
	ChannelID   string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}

func (v *VideoProps) BuildURL() string {
	return fmt.Sprintf("https://a/embed/%s?autoplay=false", v.ID)
}
