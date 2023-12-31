package logic

import (
	"errors"
	"slices"
	"sync"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/ssoroka/slice"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

func GetChannelPageProps(userId, channelId string) (*types.ChannelPageProps, error) {
	channel, cached, err := CacheChannel(channelId)
	if err != nil {
		return nil, err
	}
	channelProps := types.ChannelModelToProps(channel)
	props := types.ChannelPageProps{
		Authenticated: userId != "",
		Channel: types.ChannelWithVideosProps{
			ChannelProps: channelProps,
		},
	}

	videos, err := GetChannelVideos(24, channelId)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	if len(videos) == 0 && !cached {
		inserted, err := CacheChannelVideos(channelId)
		if err != nil {
			return nil, errors.Join(errors.New("GetChannelPageProps.CacheChannelVideos failed to cache channel videos"), err)
		}
		slices.SortFunc(inserted, func(i, j *models.Video) int {
			return int(j.PublishedAt.Unix()) - int(i.PublishedAt.Unix())
		})
		for _, v := range inserted {
			videos = append(videos, types.VideoModelToProps(v, channelProps))
		}
	}

	props.Channel.Videos = trimVideoList(24, 12, videos) // 12 can be divided by 1, 2, 3, 4 to get a nice grid

	if userId != "" && len(props.Channel.Videos) > 0 {
		var videoIds []string
		for _, v := range props.Channel.Videos {
			videoIds = append(videoIds, v.ID)
		}

		progress, err := GetUserVideoProgress(userId, videoIds...)
		if err != nil {
			return nil, errors.Join(errors.New("GetChannelPageProps.GetUserVideoProgress failed to get user progress"), err)
		}

		for i, v := range props.Channel.Videos {
			v.Progress = progress[v.ID]
			props.Channel.Videos[i] = v
		}
	}

	return &props, nil
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
