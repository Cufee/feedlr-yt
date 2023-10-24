package logic

import (
	"sync"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/api/youtube/client"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/ssoroka/slice"
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
			Channel: types.Channel{Channel: client.Channel{
				ID:          sub.ChannelID,
				URL:         channel.URL,
				Title:       channel.Title,
				Description: channel.Description,
			},
			},
			Favorite: sub.IsFavorite,
		}
		c.Thumbnail, _ = channel.Thumbnail()
		props = append(props, c)
	}

	return props, nil
}

func SearchChannels(userId, query string, limit int) ([]types.ChannelSearchResultProps, error) {
	// The search is typically a lot slower than the subscriptions query, so we run them in parallel
	var wg sync.WaitGroup

	wg.Add(1)
	var channels []client.Channel
	var channelsErr error
	go func(query string, limit int) {
		defer wg.Done()
		channels, channelsErr = youtube.C.SearchChannels(query, limit)
	}(query, limit)

	wg.Add(1)
	var subscriptions []string
	var subscriptionsErr error
	go func(userId string) {
		defer wg.Done()

		subs, err := database.C.AllUserSubscriptions(userId, database.SubscriptionGetOptions{WithChannel: true})
		subscriptionsErr = err
		for _, sub := range subs {
			subscriptions = append(subscriptions, sub.ChannelID)
		}
	}(userId)

	wg.Wait()
	if channelsErr != nil {
		return nil, channelsErr
	}
	if subscriptionsErr != nil {
		return nil, subscriptionsErr
	}

	var props []types.ChannelSearchResultProps
	for _, c := range channels {
		props = append(props, types.ChannelSearchResultProps{
			Channel:    types.Channel{Channel: c},
			Subscribed: slice.Contains(subscriptions, c.ID),
		})
		// Cache all channels to make subsequent requests faster
		go CacheChannel(c.ID)
	}

	return props, nil
}
