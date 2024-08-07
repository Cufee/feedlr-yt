package types

import (
	"fmt"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/microcosm-cc/bluemonday"
)

var policy = bluemonday.StrictPolicy()

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
			Title:       policy.Sanitize(video.Title),
			Duration:    int(video.Duration),
			Thumbnail:   fmt.Sprintf("https://i.ytimg.com/vi/%s/0.jpg", video.ID),
			PublishedAt: video.PublishedAt.Format(time.RFC3339),
			Description: policy.Sanitize(video.Description),
		},
		Channel: channel,
	}
}

func ChannelModelToProps(channel *models.Channel) ChannelProps {
	return ChannelProps{
		Channel: youtube.Channel{
			ID:          channel.ID,
			Title:       policy.Sanitize(channel.Title),
			Thumbnail:   channel.Thumbnail,
			Description: policy.Sanitize(channel.Description),
		},
		Favorite: false, // This requires an additional query to subscriptions
	}
}

func SubscriptionChannelModelToProps(sub *models.Subscription) ChannelProps {
	c := ChannelModelToProps(sub.R.Channel)
	c.Favorite = sub.Favorite
	return c
}
