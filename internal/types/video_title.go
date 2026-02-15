package types

import (
	"strings"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
)

// NormalizeVideoTitle ensures we always have a non-empty display title.
func NormalizeVideoTitle(title string, videoType youtube.VideoType, videoID string) string {
	normalized := strings.TrimSpace(title)
	if normalized != "" {
		return normalized
	}

	switch videoType {
	case youtube.VideoTypePrivate:
		return "Private video"
	case youtube.VideoTypeFailed:
		return "Unavailable video"
	default:
		return "Untitled video"
	}
}
