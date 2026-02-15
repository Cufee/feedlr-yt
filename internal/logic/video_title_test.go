package logic

import (
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
)

func TestResolveVideoTitle(t *testing.T) {
	tests := []struct {
		name      string
		incoming  string
		existing  string
		videoID   string
		videoType youtube.VideoType
		want      string
	}{
		{
			name:      "keeps incoming title",
			incoming:  "A title",
			existing:  "Old title",
			videoID:   "abc123",
			videoType: youtube.VideoTypeVideo,
			want:      "A title",
		},
		{
			name:      "uses existing title for normal videos",
			incoming:  "",
			existing:  "Cached title",
			videoID:   "abc123",
			videoType: youtube.VideoTypeVideo,
			want:      "Cached title",
		},
		{
			name:      "private video uses private fallback",
			incoming:  "",
			existing:  "Old public title",
			videoID:   "abc123",
			videoType: youtube.VideoTypePrivate,
			want:      "Private video",
		},
		{
			name:      "failed video uses unavailable fallback",
			incoming:  "",
			existing:  "Old title",
			videoID:   "abc123",
			videoType: youtube.VideoTypeFailed,
			want:      "Unavailable video",
		},
		{
			name:      "falls back to untitled",
			incoming:  "  ",
			existing:  "",
			videoID:   "abc123",
			videoType: youtube.VideoTypeVideo,
			want:      "Untitled video",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveVideoTitle(tt.incoming, tt.existing, tt.videoID, tt.videoType)
			if got != tt.want {
				t.Fatalf("resolveVideoTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
