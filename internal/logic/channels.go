package logic

import (
	"errors"

	"github.com/byvko-dev/youtube-app/internal/api/youtube"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
	"github.com/byvko-dev/youtube-app/prisma/db"
)

func CacheChannel(channelId string) (*db.ChannelModel, error) {
	exists, err := database.C.GetChannel(channelId)
	if err == nil {
		return exists, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	channel, err := youtube.Client.GetChannel(channelId)
	if err != nil {
		return nil, err
	}

	cached, err := database.C.NewChannel(channel.ID, channel.Title, channel.Thumbnail, channel.Description)
	if err != nil {
		return nil, err
	}

	return cached, nil
}

func GetUserSubscribedChannels(userId string) ([]types.ChannelProps, error) {
	subscriptions, err := database.C.AllUserSubscriptions(userId)
	if err != nil {
		return nil, err
	}

	var props []types.ChannelProps
	for _, sub := range subscriptions {
		props = append(props, types.ChannelProps{
			ID:          sub.ChannelID,
			Title:       sub.Channel().Title,
			Description: sub.Channel().Description,
			Thumbnail:   sub.Channel().Thumbnail,
			Favorite:    false,
		})
	}

	return props, nil
}

func GetChannelVideos(channels ...types.ChannelProps) ([]types.VideoProps, error) {
	var props []types.VideoProps
	for _, c := range channels {
		channel, err := database.C.GetChannel(c.ID)
		if err != nil {
			return nil, err
		}

		for _, v := range channel.Videos() {
			video := types.VideoProps{
				ID:          v.ID,
				Title:       v.Title,
				ChannelID:   v.ChannelID,
				Thumbnail:   v.Thumbnail,
				Description: v.Description,
			}
			video.URL = video.BuildURL()
			props = append(props, video)
		}
	}

	return props, nil
}

func GetUserSubscriptionsProps(userId string) ([]*types.ChannelWithVideosProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, err
	}
	subs := make(map[string]*types.ChannelWithVideosProps)
	for _, c := range channels {
		subs[c.ID] = c.WithVideos()
	}

	// Get videos for each channel and add them to the props
	videos, err := GetChannelVideos(channels...)
	if err != nil {
		return nil, err
	}
	for _, v := range videos {
		subs[v.ChannelID].Videos = append(subs[v.ChannelID].Videos, v)
	}

	var props []*types.ChannelWithVideosProps
	for _, sub := range subs {
		props = append(props, sub)
	}

	return props, nil
}
