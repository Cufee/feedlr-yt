package logic

import (
	"errors"
	"sort"
	"strings"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func NewSubscription(userId, channelId string) (*types.ChannelProps, error) {
	channel, err := CacheChannel(channelId)
	if err != nil {
		return nil, errors.Join(err, errors.New("NewSubscription.CacheChannel failed to cache channel"))
	}
	go CacheChannelVideos(channelId)

	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.Join(err, errors.New("NewSubscription.primitive.ObjectIDFromHex failed to parse userId"))
	}
	sub, err := database.DefaultClient.NewSubscription(oid, channel.ExternalID)
	if err != nil {
		return nil, errors.Join(err, errors.New("NewSubscription.database.DefaultClient.NewSubscription failed to create subscription"))
	}

	var props types.ChannelProps
	props.ID = sub.ChannelId
	props.Title = channel.Title
	props.Thumbnail = channel.Thumbnail
	props.Description = channel.Description
	props.Favorite = false

	return &props, nil
}

func DeleteSubscription(userId, channelId string) error {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return err
	}

	err = database.DefaultClient.DeleteSubscription(oid, channelId)
	if err != nil {
		return err
	}

	return nil
}

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserSubscriptionsProps(userId string) (*types.UserSubscriptionsFeedProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetUserSubscribedChannels failed to get user subscribed channels"), err)
	}

	// Sort channels by title alphabetically
	sort.Slice(channels, func(i, j int) bool {
		return strings.Compare(channels[i].Channel.Title, channels[j].Channel.Title) < 0
	})

	progress, err := GetCompleteUserProgress(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetCompleteUserProgress failed to get user progress"), err)
	}

	// Get videos for each channel and add them to the props
	var channelIds []string
	for _, c := range channels {
		channelIds = append(channelIds, c.ID)
	}
	allVideos, err := GetChannelVideos(channelIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetChannelVideos failed to get channel videos"), err)
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
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return false, errors.Join(errors.New("ToggleSubscriptionIsFavorite.primitive.ObjectIDFromHex failed to parse userId"), err)
	}

	sub, err := database.DefaultClient.FindSubscription(oid, channelId)
	if err != nil {
		return false, errors.Join(errors.New("ToggleSubscriptionIsFavorite.database.DefaultClient.FindSubscription failed to find subscription"), err)
	}

	sub.IsFavorite = !sub.IsFavorite
	err = database.DefaultClient.UpdateSubscription(sub)
	if err != nil {
		return false, errors.Join(errors.New("ToggleSubscriptionIsFavorite.database.DefaultClient.UpdateSubscription failed to update subscription"), err)
	}
	return !sub.IsFavorite, nil
}
