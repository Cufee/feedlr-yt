package webapi

import (
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/types"
)

func TestMediaDTOArtworkUsesSharedPodcastCoverAndDistinctVideoThumbnails(t *testing.T) {
	for _, tc := range []struct {
		name, id, channelID string
		videoType           youtube.VideoType
		wantArtworkURL      string
	}{
		{name: "first_episode", id: "episode-first", channelID: "show", videoType: youtube.VideoTypePodcastEpisode, wantArtworkURL: "/thumb/channel/show"},
		{name: "second_episode_same_show", id: "episode-second", channelID: "show", videoType: youtube.VideoTypePodcastEpisode, wantArtworkURL: "/thumb/channel/show"},
		{name: "different_show", id: "episode-third", channelID: "other-show", videoType: youtube.VideoTypePodcastEpisode, wantArtworkURL: "/thumb/channel/other-show"},
		{name: "first_youtube_video", id: "first-video", channelID: "creator", videoType: "video", wantArtworkURL: "/thumb/video/first-video/sddefault"},
		{name: "second_youtube_video_same_channel", id: "second-video", channelID: "creator", videoType: "video", wantArtworkURL: "/thumb/video/second-video/sddefault"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := mediaDTO(types.VideoProps{
				Video:   youtube.Video{ID: tc.id, Type: tc.videoType},
				Channel: types.ChannelProps{Channel: youtube.Channel{ID: tc.channelID}},
			})
			if item.ArtworkURL != tc.wantArtworkURL {
				t.Fatalf("artwork URL: got %q, want %q", item.ArtworkURL, tc.wantArtworkURL)
			}
			if item.Kind == "podcast" && item.ArtworkURL != item.Channel.ArtworkURL {
				t.Fatalf("podcast episode and channel artwork differ: %q and %q", item.ArtworkURL, item.Channel.ArtworkURL)
			}
		})
	}
}
