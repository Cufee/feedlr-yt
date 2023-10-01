package logic

import (
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
