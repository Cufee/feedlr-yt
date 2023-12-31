package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const UserSubscriptionCollection = "user_subscriptions"

type UserSubscription struct {
	Model `bson:",inline"`

	IsFavorite bool `json:"isFavorite" bson:"isFavorite,omitempty"`

	InternalUsers    []User             `json:"users" bson:"users,omitempty"`
	UserId           primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	InternalChannels []Channel          `json:"channels" bson:"channels,omitempty"`
	ChannelId        string             `json:"channelId" bson:"channelId,omitempty"`
}

func init() {
	addIndexHandler(UserSubscriptionCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.M{"userId": 1},
			},
			{
				Keys: bson.M{"channelId": 1},
			},
			{
				Keys: bson.D{
					{Key: "userId", Value: 1},
					{Key: "channelId", Value: 1},
				},
			},
		})
	})
}

func (model *UserSubscription) User() *User {
	if len(model.InternalUsers) > 0 {
		return &model.InternalUsers[0]
	}
	return nil
}

func (model *UserSubscription) Channel() *Channel {
	if len(model.InternalChannels) > 0 {
		return &model.InternalChannels[0]
	}
	return nil
}

type UserSubscriptionOptions struct {
	IsFavorite *bool
}

func NewUserSubscription(userId primitive.ObjectID, channelId string, opts ...UserSubscriptionOptions) *UserSubscription {
	subscription := &UserSubscription{
		UserId:    userId,
		ChannelId: channelId,
	}

	if len(opts) > 0 {
		if opts[0].IsFavorite != nil {
			subscription.IsFavorite = *opts[0].IsFavorite
		}
	}

	return subscription
}

func (subscription *UserSubscription) CollectionName() string {
	return UserSubscriptionCollection
}
