package types

import "fmt"

type Channel struct {
	ID        string
	Title     string
	Favorite  bool
	Thumbnail string
	Videos    []Video
}

type Video struct {
	ID          string
	ChannelID   string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}

func (v *Video) BuildURL() string {
	return fmt.Sprintf("https://a/embed/%s?autoplay=false", v.ID)
}
