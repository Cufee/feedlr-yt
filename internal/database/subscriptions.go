package database

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c *Client) NewSubscription(userId primitive.ObjectID, channelId string) (*models.UserSubscription, error) {
	ctx, cancel := c.Ctx()
	defer cancel()
	sub := models.NewUserSubscription(userId, channelId)
	res, err := c.Collection(models.UserSubscriptionCollection).InsertOne(ctx, sub)
	if err != nil {
		return nil, err
	}
	return sub, sub.ParseID(res.InsertedID)
}

type SubscriptionGetOptions struct {
	WithChannel bool
	WithUser    bool
}

func (c *Client) AllUserSubscriptions(userId primitive.ObjectID, opts ...SubscriptionGetOptions) ([]models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	subscriptions := []models.UserSubscription{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !options.WithChannel && !options.WithUser {
		cur, err := c.Collection(models.UserSubscriptionCollection).Find(ctx, bson.M{"userId": userId})
		if err != nil {
			return nil, err
		}
		return subscriptions, cur.All(ctx, &subscriptions)
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"userId": userId}})
	if options.WithChannel {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.ChannelCollection,
				"localField":   "channelId",
				"foreignField": "eid",
				"as":           "channels",
			}})
	}
	if options.WithUser {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserCollection,
				"localField":   "userId",
				"foreignField": "eid",
				"as":           "users",
			},
		})
	}

	cur, err := c.Collection(models.UserSubscriptionCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (c *Client) FindSubscription(userId primitive.ObjectID, channelId string, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	subscription := &models.UserSubscription{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !options.WithChannel && !options.WithUser {
		err := c.Collection(models.UserSubscriptionCollection).FindOne(ctx, bson.M{"userId": userId, "channelId": channelId}).Decode(subscription)
		return subscription, err
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"userId": userId, "channelId": channelId}})
	if options.WithChannel {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.ChannelCollection,
				"localField":   "channelId",
				"foreignField": "eid",
				"as":           "channels",
			}})
	}
	if options.WithUser {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserCollection,
				"localField":   "userId",
				"foreignField": "eid",
				"as":           "users",
			},
		})
	}

	cur, err := c.Collection(models.UserSubscriptionCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &subscription)
	if err != nil {
		return nil, err
	}

	return subscription, nil
}

func (c *Client) GetSubscription(id primitive.ObjectID, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	subscription := &models.UserSubscription{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !options.WithChannel && !options.WithUser {
		cur, err := c.Collection(models.UserSubscriptionCollection).Find(ctx, bson.M{"_id": id})
		if err != nil {
			return nil, err
		}
		return subscription, cur.All(ctx, &subscription)
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"_id": id}})
	if options.WithChannel {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.ChannelCollection,
				"localField":   "channelId",
				"foreignField": "eid",
				"as":           "channels",
			}})
	}
	if options.WithUser {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserCollection,
				"localField":   "userId",
				"foreignField": "eid",
				"as":           "users",
			},
		})
	}

	cur, err := c.Collection(models.UserSubscriptionCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &subscription)
	if err != nil {
		return nil, err
	}
	return subscription, nil
}

func (c *Client) DeleteSubscription(userId primitive.ObjectID, channelId string) error {
	ctx, cancel := c.Ctx()
	defer cancel()

	_, err := c.Collection(models.UserSubscriptionCollection).DeleteOne(ctx, bson.M{"userId": userId, "channelId": channelId})
	if errors.Is(mongo.ErrNoDocuments, err) {
		return nil
	}
	return err
}

func (c *Client) UpdateSubscription(sub *models.UserSubscription) error {
	sub.Prepare()

	ctx, cancel := c.Ctx()
	defer cancel()

	_, err := c.Collection(models.UserSubscriptionCollection).UpdateOne(ctx, bson.M{"_id": sub.ID}, bson.M{"$set": sub})
	return err
}
