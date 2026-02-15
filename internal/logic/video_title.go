package logic

import (
	"strings"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/types"
)

// resolveVideoTitle prefers a fresh non-empty title, preserves existing cached titles,
// and falls back to a safe label when no title is available.
func resolveVideoTitle(incoming, existing, videoID string, videoType youtube.VideoType) string {
	incoming = strings.TrimSpace(incoming)
	if incoming != "" {
		return incoming
	}

	existing = strings.TrimSpace(existing)
	if existing != "" && videoType != youtube.VideoTypePrivate && videoType != youtube.VideoTypeFailed {
		return existing
	}

	return types.NormalizeVideoTitle("", videoType, videoID)
}
