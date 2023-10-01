package logic

import (
	"sort"
	"strings"

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

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserSubscriptionsProps(userId string) ([]types.ChannelWithVideosProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, err
	}
	var channelIds []string
	subs := make(map[string]types.ChannelWithVideosProps)
	for _, c := range channels {
		subs[c.ID] = c.WithVideos()
		channelIds = append(channelIds, c.ID)
	}

	// Get videos for each channel and add them to the props
	videos, err := GetChannelVideos(channelIds...)
	if err != nil {
		return nil, err
	}
	for _, v := range videos {
		ch := subs[v.ChannelID]
		// Limit videos to 3
		if len(ch.Videos) >= 3 {
			continue
		}
		ch.Videos = append(subs[v.ChannelID].Videos, v)
		subs[v.ChannelID] = ch
	}

	var props []types.ChannelWithVideosProps
	for _, sub := range subs {
		props = append(props, sub)
	}

	sort.Slice(props, func(i, j int) bool {
		return strings.Compare(props[i].Channel.Title, props[j].Channel.Title) < 0
	})

	return props, nil
}
