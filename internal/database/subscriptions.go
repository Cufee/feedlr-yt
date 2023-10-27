package database

import (
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/builder"
	"go.mongodb.org/mongo-driver/bson"
)

func (c *Client) NewSubscription(userId, channelId string) (*models.UserSubscription, error) {
	channel := &models.Channel{}
	err := mgm.Coll(channel).FindByID(channelId, channel)
	if err != nil {
		return nil, err
	}

	sub := models.NewUserSubscription(userId, channelId)
	err = mgm.Coll(sub).Create(sub)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

type SubscriptionGetOptions struct {
	WithChannel bool
	WithUser    bool
}

func (c *Client) AllUserSubscriptions(userId string, opts ...SubscriptionGetOptions) ([]models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	coll := mgm.Coll(&models.UserSubscription{})
	subscriptions := []models.UserSubscription{}
	if !options.WithChannel && !options.WithUser {
		err := coll.SimpleFind(&subscriptions, bson.M{"userId": userId})
		return subscriptions, err
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"userId": userId}})
	if options.WithChannel {
		stages = append(stages, builder.Lookup(models.ChannelCollection, "channelId", "_id", "channels"))
	}
	if options.WithUser {
		stages = append(stages, builder.Lookup(models.UserCollection, "userId", "_id", "users"))
	}

	return subscriptions, coll.SimpleAggregate(&subscriptions, stages...)
}

func (c *Client) FindSubscription(userId, channelId string, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	subscription := &models.UserSubscription{}
	coll := mgm.Coll(subscription)
	if !options.WithChannel && !options.WithUser {
		err := coll.First(bson.M{"userId": userId, "channelId": channelId}, subscription)
		return subscription, err
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"userId": userId, "channelId": channelId}})
	if options.WithChannel {
		stages = append(stages, builder.Lookup(models.ChannelCollection, "channelId", "_id", "channels"))
	}
	if options.WithUser {
		stages = append(stages, builder.Lookup(models.UserCollection, "userId", "_id", "users"))
	}

	return subscription, coll.SimpleAggregate(subscription, stages...)
}

func (c *Client) GetSubscription(id string, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	subscription := &models.UserSubscription{}
	coll := mgm.Coll(subscription)
	if !options.WithChannel && !options.WithUser {
		err := coll.FindByID(id, subscription)
		return subscription, err
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"_id": id}})
	if options.WithChannel {
		stages = append(stages, builder.Lookup(models.ChannelCollection, "channelId", "_id", "channels"))
	}
	if options.WithUser {
		stages = append(stages, builder.Lookup(models.UserCollection, "userId", "_id", "users"))
	}

	return subscription, coll.SimpleAggregate(subscription, stages...)
}

func (c *Client) DeleteSubscription(userId, channelId string) error {
	_, err := mgm.Coll(&models.UserSubscription{}).DeleteOne(mgm.Ctx(), bson.M{"userId": userId, "channelId": channelId})
	return err
}

func (c *Client) ToggleSubscriptionIsFavorite(id string) (*models.UserSubscription, error) {
	sub, err := c.GetSubscription(id)
	if err != nil {
		return nil, err
	}
	sub.IsFavorite = !sub.IsFavorite
	return sub, mgm.Coll(sub).Update(sub)
}
