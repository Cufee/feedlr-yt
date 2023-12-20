package logic

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func FindSubscription(userId, channelId string) (bool, error) {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return false, errors.Join(err, errors.New("NewSubscription.primitive.ObjectIDFromHex failed to parse userId"))
	}
	_, err = database.DefaultClient.FindSubscription(oid, channelId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, errors.Join(err, errors.New("NewSubscription.database.DefaultClient.NewSubscription failed to create subscription"))
	}
	return true, nil
}

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
