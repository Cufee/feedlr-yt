package types

import (
	"fmt"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func VideoToProps(video youtube.Video, channel ChannelProps) VideoProps {
	video.Title = NormalizeVideoTitle(video.Title, video.Type, video.ID)
	return VideoProps{
		Video:   video,
		Channel: channel,
	}
}

func VideoModelToProps(video *models.Video, channel ChannelProps) VideoProps {
	videoType := youtube.VideoType(video.Type)
	props := VideoProps{
		Video: youtube.Video{
			Type:        videoType,
			ID:          video.ID,
			Title:       NormalizeVideoTitle(video.Title, videoType, video.ID),
			Duration:    int(video.Duration),
			Description: video.Description,
		},
		PublishedAt: video.PublishedAt,
		CreatedAt:   video.CreatedAt,
		Channel:     channel,
	}

	if videoType == youtube.VideoTypePodcastEpisode {
		// Podcast episodes use the show artwork instead of a video thumbnail.
		props.Video.Thumbnail = channel.Thumbnail
		props.StreamURL = "/media/stream/" + video.ID
	} else {
		props.Video.Thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/0.jpg", video.ID)
	}

	return props
}

func ChannelModelToProps(channel *models.Channel) ChannelProps {
	return ChannelProps{
		Channel: youtube.Channel{
			ID:          channel.ID,
			Title:       channel.Title,
			Thumbnail:   channel.Thumbnail,
			Description: channel.Description,
		},
		Favorite:      false, // This requires an additional query to subscriptions
		FeedUpdatedAt: channel.FeedUpdatedAt,
		IsPodcast:     channel.R.GetPodcastShow() != nil,
	}
}

func SubscriptionChannelModelToProps(sub *models.Subscription) ChannelProps {
	c := ChannelModelToProps(sub.R.Channel)
	c.Favorite = sub.Favorite
	c.VideoFilter = VideoFilter(sub.VideoFilter)
	return c
}

func PlaylistModelToProps(p *models.Playlist, count int, progress int, thumbnailVideoID string) PlaylistProps {
	return PlaylistProps{
		ID:                p.ID,
		Name:              p.Name,
		Description:       p.Description,
		Slug:              p.Slug,
		VideoCount:        count,
		Progress:          progress,
		ThumbnailVideoID:  thumbnailVideoID,
		YouTubePlaylistID: p.YoutubePlaylistID.String,
		UpdatedAt:         p.UpdatedAt,
	}
}

func PasskeyToProps(record *models.Passkey) PasskeyProps {
	return PasskeyProps{
		ID:        record.ID,
		Label:     record.Label,
		CreatedAt: record.CreatedAt,
	}
}
