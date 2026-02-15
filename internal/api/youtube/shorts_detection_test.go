package youtube

import (
	"testing"

	ytv3 "google.golang.org/api/youtube/v3"
)

func TestLooksLikeShortsMetadata(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   bool
	}{
		{name: "hash shorts", values: []string{"Great clip #shorts"}, want: true},
		{name: "hash short", values: []string{"#short quick update"}, want: true},
		{name: "shorts url", values: []string{"watch here: /shorts/abc123"}, want: true},
		{name: "plain text", values: []string{"Regular weekly update"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeShortsMetadata(tt.values...)
			if got != tt.want {
				t.Fatalf("looksLikeShortsMetadata()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestThumbnailDetailsPortrait(t *testing.T) {
	t.Run("portrait thumbnail", func(t *testing.T) {
		got := thumbnailDetailsPortrait(&ytv3.ThumbnailDetails{
			Medium: &ytv3.Thumbnail{Width: 405, Height: 720},
		})
		if !got {
			t.Fatal("expected portrait thumbnail to be detected as short")
		}
	})

	t.Run("landscape thumbnail", func(t *testing.T) {
		got := thumbnailDetailsPortrait(&ytv3.ThumbnailDetails{
			Medium: &ytv3.Thumbnail{Width: 1280, Height: 720},
		})
		if got {
			t.Fatal("expected landscape thumbnail to not be detected as short")
		}
	})
}

func TestPlaylistItemLikelyShort(t *testing.T) {
	tests := []struct {
		name string
		item *ytv3.PlaylistItem
		want bool
	}{
		{
			name: "shorts hashtag",
			item: &ytv3.PlaylistItem{
				Snippet: &ytv3.PlaylistItemSnippet{
					Title: "clip #shorts",
				},
			},
			want: true,
		},
		{
			name: "portrait thumbnail",
			item: &ytv3.PlaylistItem{
				Snippet: &ytv3.PlaylistItemSnippet{
					Title: "clip",
					Thumbnails: &ytv3.ThumbnailDetails{
						Default: &ytv3.Thumbnail{Width: 360, Height: 640},
					},
				},
			},
			want: true,
		},
		{
			name: "regular item",
			item: &ytv3.PlaylistItem{
				Snippet: &ytv3.PlaylistItemSnippet{
					Title: "regular upload",
					Thumbnails: &ytv3.ThumbnailDetails{
						Default: &ytv3.Thumbnail{Width: 640, Height: 360},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := playlistItemLikelyShort(tt.item)
			if got != tt.want {
				t.Fatalf("playlistItemLikelyShort()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoIsShortFallback(t *testing.T) {
	tests := []struct {
		name string
		v    Video
		want bool
	}{
		{name: "explicit short type", v: Video{Type: VideoTypeShort, Duration: 170}, want: true},
		{name: "legacy short duration", v: Video{Type: VideoTypeVideo, Duration: 45}, want: true},
		{name: "shorts metadata up to 180s", v: Video{Type: VideoTypeVideo, Duration: 170, Title: "clip #shorts"}, want: true},
		{name: "regular short video not tagged", v: Video{Type: VideoTypeVideo, Duration: 170, Title: "quick update"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.isShort()
			if got != tt.want {
				t.Fatalf("Video.isShort()=%v, want %v", got, tt.want)
			}
		})
	}
}
