package types

import (
	"fmt"
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
			ID:          video.ID,
			Title:       video.Title,
			Duration:    int(video.Duration),
			Thumbnail:   fmt.Sprintf("https://i.ytimg.com/vi/%s/maxresdefault.jpg", video.ID),
			PublishedAt: video.PublishedAt.Format(time.RFC3339),
			Description: video.Description,
		},
		Channel: channel,
	}
}

func ChannelModelToProps(channel *models.Channel) ChannelProps {
	return ChannelProps{
		Channel: youtube.Channel{
			ID:          channel.ID,
			Title:       channel.Title,
			Thumbnail:   channel.Thumbnail,
			Description: channel.Description,
		},
		Favorite: false, // This requires an additional query to subscriptions
	}
}

func SubscriptionChannelModelToProps(sub *models.Subscription) ChannelProps {
	c := ChannelModelToProps(sub.R.Channel)
	c.Favorite = sub.Favorite
	return c
}
