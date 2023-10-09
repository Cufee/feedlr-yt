package logic

import (
	"sort"
	"strings"

	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
)

func NewSubscription(userId, channelId string) (*types.ChannelProps, error) {
	_, err := CacheChannel(channelId)
	if err != nil {
		return nil, err
	}

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

	progress, err := GetCompleteUserProgress(userId)
	if err != nil {
		return nil, err
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
		v.Progress = progress[v.ID]
		ch.Videos = append(subs[v.ChannelID].Videos, v)
		subs[v.ChannelID] = ch
	}

	var props []types.ChannelWithVideosProps
	for _, sub := range subs {
		sub.CaughtUp = true
		for _, v := range sub.Videos {
			if v.Progress < 1 {
				sub.CaughtUp = false
				break
			}
		}

		props = append(props, sub)
	}

	sort.Slice(props, func(i, j int) bool {
		if props[i].CaughtUp {
			return false
		}
		if props[j].CaughtUp {
			return true
		}
		return strings.Compare(props[i].Channel.Title, props[j].Channel.Title) < 0
	})

	return props, nil
}
