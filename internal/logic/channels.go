package logic

import (
	"context"

	"slices"

	"github.com/cufee/feedlr-yt/internal/api/piped"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/pkg/errors"
	"github.com/ssoroka/slice"
	"golang.org/x/sync/errgroup"
)

/*
Returns a list of channel props for all user subscriptions
*/
func GetUserSubscribedChannels(ctx context.Context, db database.SubscriptionsClient, userID string) ([]types.ChannelProps, error) {
	subscriptions, err := db.UserSubscriptions(ctx, userID, database.Subscription{}.WithChannel())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get subscriptions")
	}

	var props []types.ChannelProps
	for _, sub := range subscriptions {
		channel := sub.R.Channel
		if channel == nil {
			continue
		}
		props = append(props, types.SubscriptionChannelModelToProps(sub))
	}
	return props, nil
}

func GetChannelPageProps(ctx context.Context, db database.Client, ppd *piped.Client, userID, channelID string) (*types.ChannelPageProps, error) {
	channel, cached, err := CacheChannel(ctx, db, ppd, channelID)
	if err != nil {
		return nil, err
	}
	channelProps := types.ChannelModelToProps(channel)
	props := types.ChannelPageProps{
		Authenticated: userID != "",
		Channel: types.ChannelWithVideosProps{
			ChannelProps: channelProps,
		},
	}

	videos, err := GetChannelVideos(ctx, db, 24, channelID)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, err
	}

	if len(videos) == 0 && !cached {
		inserted, err := CacheChannelVideos(ctx, db, ppd, channelID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to cache channel videos")
		}
		slices.SortFunc(inserted, func(i, j *models.Video) int {
			return int(j.PublishedAt.Unix()) - int(i.PublishedAt.Unix())
		})
		for _, v := range inserted {
			videos = append(videos, types.VideoModelToProps(v, channelProps))
		}
	}

	props.Channel.Videos = trimVideoList(24, 12, videos) // 12 can be divided by 1, 2, 3, 4 to get a nice grid

	if userID != "" && len(props.Channel.Videos) > 0 {
		var videoIds []string
		for _, v := range props.Channel.Videos {
			videoIds = append(videoIds, v.ID)
		}

		views, err := GetUserViews(ctx, db, userID, videoIds...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get user progress")
		}

		for i, v := range props.Channel.Videos {
			if view, ok := views[v.ID]; ok {
				v.Hidden = view.Hidden.Bool
				v.Progress = int(view.Progress)
			}
			props.Channel.Videos[i] = v
		}
	}

	return &props, nil
}

func SearchChannels(
	ctx context.Context,
	db database.Client,
	ppd *piped.Client,
	userID string,
	query string,
	limit int,
) ([]types.ChannelSearchResultProps, error) {
	var group errgroup.Group

	var remoteChannels []piped.SearchItem
	var subscribedChannels []string

	group.Go(func() error {
		channels, err := ppd.SearchChannels(ctx, query)
		if err != nil {
			return err
		}
		remoteChannels = channels
		return nil

	})

	group.Go(func() error {
		subscriptions, err := db.UserSubscriptions(ctx, userID)
		if err != nil {
			return err
		}
		for _, c := range subscriptions {
			subscribedChannels = append(subscribedChannels, c.ChannelID)
		}
		return nil
	})

	err := group.Wait()
	if err != nil {
		return nil, err
	}

	var props []types.ChannelSearchResultProps
	for _, c := range remoteChannels {
		props = append(props, types.ChannelSearchResultProps{
			ChannelProps: types.ChannelProps{
				ID:          c.ChannelID(),
				Name:        c.Name,
				Description: c.Description,
				Thumbnail:   c.Thumbnail,
			},
			Subscribed: slice.Contains(subscribedChannels, c.ChannelID()),
		})
	}

	return props, nil
}
