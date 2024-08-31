package piped

import "strings"

type SearchItem struct {
	URL         string `json:"url"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	Verified    bool   `json:"verified"`
}

func (item SearchItem) ChannelID() string {
	return strings.TrimPrefix(item.URL, "/channel/")
}
