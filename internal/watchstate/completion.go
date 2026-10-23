package watchstate

// CompletionBufferSeconds is shared by collections, web playback and TV sync.
func CompletionBufferSeconds(duration int) int {
	switch {
	case duration > 30*60:
		return 5 * 60
	case duration > 15*60:
		return 60
	default:
		return 30
	}
}

// IsCompleted excludes untouched items and media with an unknown duration.
func IsCompleted(progress, duration int) bool {
	return progress > 0 && duration > 0 && progress+CompletionBufferSeconds(duration) >= duration
}
