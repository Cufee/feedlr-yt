package logic

import (
	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
)

/*
Returns a list of channel props for all user subscriptions
*/
func GetUserSubscribedChannels(userId string) ([]types.ChannelProps, error) {
	subscriptions, err := database.C.AllUserSubscriptions(userId)
	if err != nil {
		return nil, err
	}

	var props []types.ChannelProps
	for _, sub := range subscriptions {
		c := types.ChannelProps{
			Channel: client.Channel{
				ID:          sub.ChannelID,
				URL:         sub.Channel().URL,
				Title:       sub.Channel().Title,
				Description: sub.Channel().Description,
			},
			Favorite: sub.IsFavorite,
		}
		c.Thumbnail, _ = sub.Channel().Thumbnail()
		props = append(props, c)
	}

	return props, nil
}

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(channelIds ...string) ([]types.VideoProps, error) {
	videos, err := database.C.GetVideosByChannelID(channelIds...)
	if err != nil {
		return nil, err
	}

	var props []types.VideoProps
	for _, vid := range videos {
		v := types.VideoProps{
			Video: client.Video{
				ID:          vid.ID,
				URL:         vid.URL,
				Title:       vid.Title,
				Description: vid.Description,
			},
			ChannelID: vid.ChannelID,
		}
		v.Thumbnail, _ = vid.Thumbnail()
		props = append(props, v)
	}

	return props, nil
}

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserSubscriptionsProps(userId string) ([]*types.ChannelWithVideosProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, err
	}
	var channelIds []string
	subs := make(map[string]*types.ChannelWithVideosProps)
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
		subs[v.ChannelID].Videos = append(subs[v.ChannelID].Videos, v)
	}

	var props []*types.ChannelWithVideosProps
	for _, sub := range subs {
		props = append(props, sub)
	}

	return props, nil
}
