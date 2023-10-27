package database

import (
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/builder"
	"go.mongodb.org/mongo-driver/bson"
)

type ChannelGetOptions struct {
	WithVideos        bool
	WithSubscriptions bool
}

func (c *Client) GetAllChannels(opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	coll := mgm.Coll(&models.Channel{})
	channels := []models.Channel{}

	if !options.WithVideos && !options.WithSubscriptions {
		err := coll.SimpleFind(&channels, bson.M{})
		if err != nil {
			return nil, err
		}
		return channels, nil
	}

	var stages []interface{}
	if options.WithVideos {
		stages = append(stages, builder.Lookup(models.VideoCollection, "_id", "channelId", "videos"))
	}
	if options.WithSubscriptions {
		stages = append(stages, builder.Lookup(models.UserSubscriptionCollection, "_id", "channelId", "subscriptions"))
	}

	return channels, coll.SimpleAggregate(&channels, stages...)
}

func (c *Client) GetAllChannelsWithSubscriptions(opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	var stages []interface{}
	coll := mgm.Coll(&models.Channel{})
	channels := []models.Channel{}

	stages = append(stages, builder.Lookup(models.UserSubscriptionCollection, "_id", "channelId", "subscriptions"))
	if options.WithVideos {
		stages = append(stages, builder.Lookup(models.VideoCollection, "_id", "channelId", "videos"))
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

	return channels, coll.SimpleAggregate(&channels, stages...)
}

func (c *Client) GetChannel(channelId string, opts ...ChannelGetOptions) (*models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	channel := &models.Channel{}
	coll := mgm.Coll(channel)
	if !options.WithVideos && !options.WithSubscriptions {
		err := coll.FindByID(channelId, channel)
		if err != nil {
			return nil, err
		}
		return channel, nil
	}

	var stages []interface{}
	if options.WithVideos {
		stages = append(stages, builder.Lookup(models.VideoCollection, "_id", "channelId", "videos"))
	}
	if options.WithSubscriptions {
		stages = append(stages, builder.Lookup(models.UserSubscriptionCollection, "_id", "channelId", "subscriptions"))
	}

	return channel, coll.SimpleAggregate(channel, stages...)
}

func (c *Client) GetChannelsByID(channelIds []string, opts ...ChannelGetOptions) ([]models.Channel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	coll := mgm.Coll(&models.Channel{})
	channels := []models.Channel{}
	if !options.WithVideos && !options.WithSubscriptions {
		err := coll.SimpleFind(&channels, bson.M{"_id": bson.M{"$in": channelIds}})
		if err != nil {
			return nil, err
		}
		return channels, nil
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"_id": bson.M{"$in": channelIds}}})
	if options.WithVideos {
		stages = append(stages, builder.Lookup(models.VideoCollection, "_id", "channelId", "videos"))
	}
	if options.WithSubscriptions {
		stages = append(stages, builder.Lookup(models.UserSubscriptionCollection, "_id", "channelId", "subscriptions"))
	}

	return channels, coll.SimpleAggregate(&channels, stages...)
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
	err := mgm.Coll(channel).Create(channel)
	if err != nil {
		return nil, err
	}
	channel.Subscriptions = []models.UserSubscription{}
	channel.Videos = []models.Video{}
	return channel, nil
}
