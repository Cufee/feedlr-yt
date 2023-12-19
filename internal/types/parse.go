package types

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func VideoToProps(video youtube.Video, channel ChannelProps) VideoProps {
	return VideoProps{
		Video:   video,
		Channel: channel,
	}
}

func VideoModelToProps(video *models.Video, channel ChannelProps) VideoProps {
	return VideoProps{
		Video: youtube.Video{
			Type:        youtube.VideoType(video.Type),
			ID:          video.ExternalID,
			URL:         video.URL,
			Title:       video.Title,
			Duration:    video.Duration,
			Thumbnail:   video.Thumbnail,
			PublishedAt: video.PublishedAt.Format(time.RFC3339),
			Description: video.Description,
		},
		Channel: channel,
	}
}

func ChannelModelToProps(channel *models.Channel) ChannelProps {
	return ChannelProps{
		Channel: youtube.Channel{
			ID:          channel.ExternalID,
			URL:         channel.URL,
			Title:       channel.Title,
			Thumbnail:   channel.Thumbnail,
			Description: channel.Description,
		},
		Favorite: false, // This requires an additional query
	}
}
