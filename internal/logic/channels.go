package logic

import (
	"github.com/byvko-dev/youtube-app/internal/api/youtube"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
)

/*
Returns a list of channel props for all user subscriptions
*/
func GetUserSubscribedChannels(userId string) ([]types.ChannelProps, error) {
	subscriptions, err := database.C.AllUserSubscriptions(userId, database.SubscriptionGetOptions{WithChannel: true})
	if err != nil {
		return nil, err
	}

	var props []types.ChannelProps
	for _, sub := range subscriptions {
		channel := sub.Channel()
		c := types.ChannelProps{
			Channel: client.Channel{
				ID:          sub.ChannelID,
				URL:         channel.URL,
				Title:       channel.Title,
				Description: channel.Description,
			},
			Favorite: sub.IsFavorite,
		}
		c.Thumbnail, _ = channel.Thumbnail()
		props = append(props, c)
	}

	return props, nil
}

func SearchChannels(query string, limit int) ([]client.Channel, error) {
	channels, err := youtube.C.SearchChannels(query, limit)
	if err != nil {
		return nil, err
	}

	// Cache all channels to make subsequent requests faster
	for _, c := range channels {
		go CacheChannel(c.ID)
	}

	return channels, nil
}
