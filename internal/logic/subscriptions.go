package logic

import (
	"sort"
	"strings"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
)

func NewSubscription(userId, channelId string) (*types.ChannelProps, error) {
	_, err := CacheChannel(channelId)
	if err != nil {
		return nil, err
	}
	go CacheChannelVideos(channelId)

	sub, err := database.C.NewSubscription(userId, channelId)
	if err != nil {
		return nil, err
	}

	var props types.ChannelProps
	props.ID = sub.ChannelID
	props.Title = sub.Channel().Title
	props.Description = sub.Channel().Description
	props.Thumbnail, _ = sub.Channel().Thumbnail()
	props.Favorite = false

	return &props, nil
}

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserSubscriptionsProps(userId string) (*types.UserSubscriptionsFeedProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, err
	}

	// Sort channels by title alphabetically
	sort.Slice(channels, func(i, j int) bool {
		return strings.Compare(channels[i].Channel.Title, channels[j].Channel.Title) < 0
	})

	progress, err := GetCompleteUserProgress(userId)
	if err != nil {
		return nil, err
	}

	// Get videos for each channel and add them to the props
	var channelIds []string
	for _, c := range channels {
		channelIds = append(channelIds, c.ID)
	}
	allVideos, err := GetChannelVideos(channelIds...)
	if err != nil {
		return nil, err
	}

	// Map videos to channel IDs
	channelVideos := make(map[string][]types.VideoProps)
	for _, v := range allVideos {
		// Limit videos to 3
		if len(channelVideos[v.ChannelID]) >= 3 {
			continue
		}
		v.Progress = progress[v.ID]
		channelVideos[v.ChannelID] = append(channelVideos[v.ChannelID], v)
	}

	// Sort channels for subscription feed props
	var subscriptions types.UserSubscriptionsFeedProps
	for _, channel := range channels {
		videos := channelVideos[channel.ID]
		props := channel.WithVideos(videos...)

		props.CaughtUp = true
		for _, v := range props.Videos {
			if v.Progress < 1 {
				props.CaughtUp = false
				break
			}
		}

		if props.CaughtUp {
			subscriptions.WithoutNewVideos = append(subscriptions.WithoutNewVideos, props)
		} else if props.Favorite {
			subscriptions.Favorites = append(subscriptions.Favorites, props)
		} else {
			subscriptions.WithNewVideos = append(subscriptions.WithNewVideos, props)
		}
		subscriptions.All = append(subscriptions.All, props)
	}

	return &subscriptions, nil
}

func ToggleSubscriptionIsFavorite(userId, channelId string) (bool, error) {
	sub, err := database.C.FindSubscription(userId, channelId)
	if err != nil {
		return false, err
	}

	update, err := database.C.ToggleSubscriptionIsFavorite(sub.ID)
	if err != nil {
		return false, err
	}
	return update.IsFavorite, nil
}
