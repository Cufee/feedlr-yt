package logic

import (
	"errors"
	"sync"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/ssoroka/slice"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/*
Returns a list of channel props for all user subscriptions
*/
func GetUserSubscribedChannels(userId string) ([]types.ChannelProps, error) {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscribedChannels.primitive.ObjectIDFromHex failed to parse userId"), err)
	}

	subscriptions, err := database.DefaultClient.AllUserSubscriptions(oid, database.SubscriptionGetOptions{WithChannel: true})
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscribedChannels.database.DefaultClient.AllUserSubscriptions failed to get subscriptions"), err)
	}

	var props []types.ChannelProps
	for _, sub := range subscriptions {
		channel := sub.Channel()
		if channel == nil {
			continue
		}
		c := types.ChannelProps{
			Channel: youtube.Channel{
				ID:          channel.ExternalID,
				URL:         channel.URL,
				Title:       channel.Title,
				Thumbnail:   channel.Thumbnail,
				Description: channel.Description,
			},
			Favorite: sub.IsFavorite,
		}
		props = append(props, c)
	}

	return props, nil
}

func SearchChannels(userId, query string, limit int) ([]types.ChannelSearchResultProps, error) {
	// The search is typically a lot slower than the subscriptions query, so we run them in parallel
	var wg sync.WaitGroup

	wg.Add(1)
	var channels []youtube.Channel
	var channelsErr error
	go func(query string, limit int) {
		defer wg.Done()
		channels, channelsErr = youtube.DefaultClient.SearchChannels(query, limit)
	}(query, limit)

	wg.Add(1)
	var subscriptions []string
	var subscriptionsErr error
	go func(userId string) {
		defer wg.Done()

		oid, err := primitive.ObjectIDFromHex(userId)
		if err != nil {
			subscriptionsErr = err
			return
		}

		subs, err := database.DefaultClient.AllUserSubscriptions(oid, database.SubscriptionGetOptions{WithChannel: true})
		subscriptionsErr = err
		for _, sub := range subs {
			subscriptions = append(subscriptions, sub.ChannelId)
		}
	}(userId)

	wg.Wait()
	if channelsErr != nil {
		return nil, errors.Join(errors.New("SearchChannels.youtube.DefaultClient.SearchChannels failed to search channels"), channelsErr)
	}
	if subscriptionsErr != nil {
		return nil, errors.Join(errors.New("SearchChannels.database.DefaultClient.AllUserSubscriptions failed to get subscriptions"), subscriptionsErr)
	}

	var props []types.ChannelSearchResultProps
	for _, c := range channels {
		props = append(props, types.ChannelSearchResultProps{
			Channel:    c,
			Subscribed: slice.Contains(subscriptions, c.ID),
		})
		// Cache all channels to make subsequent requests faster
		go CacheChannel(c.ID)
	}

	return props, nil
}
