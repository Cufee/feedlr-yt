package database

import (
	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ChannelGetOptions struct {
	WithVideos        bool
	VideosLimit       int
	WithSubscriptions bool
}

func (c *Client) GetAllChannels(opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	ctx, cancel := c.Ctx()
	defer cancel()

	channels := []models.Channel{}

	if !options.WithVideos && !options.WithSubscriptions {
		cur, err := c.Collection(models.ChannelCollection).Find(ctx, bson.M{})
		if err != nil {
			return nil, err
		}
		return channels, cur.All(ctx, &channels)
	}

	var stages []interface{}
	if options.WithVideos {
		lookup := bson.M{
			"from":         models.VideoCollection,
			"localField":   "eid",
			"foreignField": "channelId",
			"as":           "videos",
		}
		if options.VideosLimit > 0 {
			lookup["let"] = bson.M{"indicator_id": "$eid"}
			lookup["pipeline"] = mongo.Pipeline{
				{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$eq", Value: bson.A{"$channelId", "$$indicator_id"}}}}}}},
				{{Key: "$limit", Value: options.VideosLimit}},
			}
		}
		stages = append(stages, bson.M{"$lookup": lookup})
	}
	if options.WithSubscriptions {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserSubscriptionCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "subscriptions",
			},
		})
	}

	cur, err := c.Collection(models.ChannelCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) GetAllChannelsWithSubscriptions(opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	var stages []interface{}
	channels := []models.Channel{}

	stages = append(stages, bson.M{
		"$lookup": bson.M{
			"from":         models.UserSubscriptionCollection,
			"localField":   "eid",
			"foreignField": "channelId",
			"as":           "subscriptions",
		},
	})
	if options.WithVideos {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.VideoCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "videos",
			},
		})
	}

	// subscriptions > 0
	stages = append(stages, bson.M{
		"$match": bson.M{
			"subscriptions": bson.M{
				"$exists": true,
				"$ne":     bson.A{},
			},
		},
	})

	ctx, cancel := c.Ctx()
	defer cancel()
	cur, err := c.Collection(models.ChannelCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) GetChannel(channelId string, opts ...ChannelGetOptions) (*models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	channel := &models.Channel{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !options.WithVideos && !options.WithSubscriptions {
		return channel, c.Collection(models.ChannelCollection).FindOne(ctx, bson.M{"eid": channelId}).Decode(channel)
	}

	var stages []interface{}
	if options.WithVideos {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.VideoCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "videos",
			},
		})
	}
	if options.WithSubscriptions {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserSubscriptionCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "subscriptions",
			},
		})
	}

	cur, err := c.Collection(models.ChannelCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.Decode(channel)
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (c *Client) GetChannelsByID(channelIds []string, opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	channels := []models.Channel{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !options.WithVideos && !options.WithSubscriptions {
		cur, err := c.Collection(models.ChannelCollection).Find(ctx, bson.M{"eid": bson.M{"$in": channelIds}})
		if err != nil {
			return nil, err
		}
		return channels, cur.All(ctx, &channels)
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"eid": bson.M{"$in": channelIds}}})
	if options.WithVideos {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.VideoCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "videos",
			},
		})
	}
	if options.WithSubscriptions {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserSubscriptionCollection,
				"localField":   "eid",
				"foreignField": "channelId",
				"as":           "subscriptions",
			},
		})
	}

	cur, err := c.Collection(models.ChannelCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}

	err = cur.All(ctx, &channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

type ChannelCreateModel struct {
	ID          string
	URL         string
	Title       string
	Description string
	Thumbnail   string
}

func (c *Client) NewChannel(ch ChannelCreateModel) (*models.Channel, error) {
	channel := models.NewChannel(ch.ID, ch.URL, ch.Title, models.ChannelOptions{Thumbnail: &ch.Thumbnail, Description: &ch.Description})
	channel.Prepare()

	ctx, cancel := c.Ctx()
	defer cancel()
	res, err := c.Collection(models.ChannelCollection).InsertOne(ctx, channel)
	if err != nil {
		return nil, err
	}
	channel.ParseID(res.InsertedID)
	channel.Subscriptions = []models.UserSubscription{}
	channel.Videos = []models.Video{}
	return channel, nil
}
