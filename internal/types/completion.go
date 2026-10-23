package types

import "github.com/cufee/feedlr-yt/internal/watchstate"

// CompletionBufferSeconds is shared by collections, web playback and TV sync.
func CompletionBufferSeconds(duration int) int {
	return watchstate.CompletionBufferSeconds(duration)
}

// IsCompleted never treats an untouched item or an unknown duration as complete.
func IsCompleted(progress, duration int) bool {
	return watchstate.IsCompleted(progress, duration)
}
