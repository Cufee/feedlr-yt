package types

import (
	"os"

	"github.com/cufee/feedlr-yt/internal/api/piped"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

var apiHost = os.Getenv("PIPED_API_URL")

func VideoToProps(video piped.Video, channel ChannelProps) VideoProps {
	return VideoProps{
		ID:          video.ID,
		Title:       video.Title,
		Description: video.Description,
		Type:        video.Type,
		Duration:    int(video.Duration),
		PublishedAt: video.PublishDate(),
		Thumbnail:   video.Thumbnail,
		Channel:     channel,
	}
}

func VideoModelToProps(video *models.Video, channel ChannelProps) VideoProps {
	return VideoProps{
		ID:          video.ID,
		Title:       video.Title,
		Description: video.Description,
		Type:        video.Type,
		Duration:    int(video.Duration),
		PublishedAt: video.PublishedAt,
		Thumbnail:   video.Thumbnail,
		Channel:     channel,
	}
}

func ChannelModelToProps(channel *models.Channel) ChannelProps {
	return ChannelProps{
		ID:          channel.ID,
		Name:        channel.Title,
		Description: channel.Description,
		Thumbnail:   channel.Thumbnail,
		Favorite:    false, // This requires an additional query to subscriptions
	}
}

func SubscriptionChannelModelToProps(sub *models.Subscription) ChannelProps {
	c := ChannelModelToProps(sub.R.Channel)
	c.Favorite = sub.Favorite
	return c
}

func PasskeyToProps(record *models.Passkey) PasskeyProps {
	return PasskeyProps{
		ID:        record.ID,
		Label:     record.Label,
		CreatedAt: record.CreatedAt,
	}
}
