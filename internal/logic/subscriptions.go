package logic

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/pkg/errors"
)

func SubscriptionExists(ctx context.Context, db database.SubscriptionsClient, userId, channelId string) (bool, error) {
	_, err := db.FindSubscription(ctx, userId, channelId)
	if err != nil {
		if database.IsErrNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to create subscription")
	}
	return true, nil
}

func NewSubscription(ctx context.Context, db database.Client, userId, channelId string) (*types.ChannelProps, error) {
	channel, _, err := CacheChannel(ctx, db, channelId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to cache channel")
	}

	go func() {
		if time.Since(channel.FeedUpdatedAt) < time.Hour {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
		defer cancel()
		CacheChannelVideos(ctx, db, 3, channelId)
	}()

	sub, err := db.NewSubscription(ctx, userId, channel.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create subscription")
	}

	var props types.ChannelProps
	props.ID = sub.ChannelID
	props.Title = channel.Title
	props.Thumbnail = channel.Thumbnail
	props.Description = channel.Description
	props.Favorite = false

	return &props, nil
}

func DeleteSubscription(ctx context.Context, db database.SubscriptionsClient, userId, channelId string) error {
	err := db.DeleteSubscription(ctx, userId, channelId)
	if err != nil {
		return err
	}
	return nil
}
