package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const ChannelCollection = "channels"

type Channel struct {
	Model      `bson:",inline"`
	ExternalID string `json:"eid" bson:"eid"`

	URL         string `json:"url" bson:"url" field:"required"`
	Title       string `json:"title" bson:"title" field:"required"`
	Thumbnail   string `json:"thumbnail" bson:"thumbnail"`
	Description string `json:"description" bson:"description"`

	Videos        []Video            `json:"videos" bson:"videos,omitempty"`
	Subscriptions []UserSubscription `json:"subscriptions" bson:"subscriptions,omitempty"`
}

func init() {
	addIndexHandler(ChannelCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys:    bson.M{"eid": 1},
				Options: &options.IndexOptions{Unique: &[]bool{true}[0], Name: &[]string{"eid"}[0]},
			},
		})
	})
}

type ChannelOptions struct {
	Thumbnail   *string
	Description *string
}

func NewChannel(id, url, title string, opts ...ChannelOptions) *Channel {
	var thumbnail, description string
	if len(opts) > 0 {
		if opts[0].Thumbnail != nil {
			thumbnail = *opts[0].Thumbnail
		}
		if opts[0].Description != nil {
			description = *opts[0].Description
		}
	}

	channel := Channel{
		ExternalID:  id,
		URL:         url,
		Title:       title,
		Thumbnail:   thumbnail,
		Description: description,
	}
	return &channel
}

func (c *Channel) CollectionName() string {
	return ChannelCollection
}
