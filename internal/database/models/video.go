package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const VideoCollection = "videos"

type Video struct {
	Model      `bson:",inline"`
	ExternalID string `json:"eid" bson:"eid,omitempty"`

	URL         string    `json:"url" bson:"url,omitempty"`
	Title       string    `json:"title" bson:"title,omitempty"`
	Duration    int       `json:"duration" bson:"duration,omitempty"`
	Thumbnail   string    `json:"thumbnail" bson:"thumbnail,omitempty"`
	Description string    `json:"description" bson:"description,omitempty"`
	PublishedAt time.Time `json:"publishedAt" bson:"publishedAt,omitempty"`

	Views     []VideoView `json:"views" bson:"views,omitempty"`
	Channel   *Channel    `json:"channel" bson:"channel,omitempty"`
	ChannelId string      `json:"channelId" bson:"channelId,omitempty"`
}

func init() {
	addIndexHandler(VideoCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.M{"eid": 1},
			},
			{
				Keys: bson.M{"channelId": 1},
			},
			{
				Keys: bson.D{
					{Key: "channelId", Value: 1},
					{Key: "publishedAt", Value: -1},
				},
			},
		})
	})
}

type VideoOptions struct {
	Duration    *int
	Thumbnail   *string
	Description *string
}

func NewVideo(id, url, title, channelId string, publishedAt time.Time, opts ...VideoOptions) *Video {
	var duration int
	var thumbnail, description string
	if len(opts) > 0 {
		if opts[0].Duration != nil {
			duration = *opts[0].Duration
		}
		if opts[0].Thumbnail != nil {
			thumbnail = *opts[0].Thumbnail
		}
		if opts[0].Description != nil {
			description = *opts[0].Description
		}
	}

	video := Video{
		ExternalID:  id,
		URL:         url,
		Title:       title,
		Duration:    duration,
		Thumbnail:   thumbnail,
		PublishedAt: publishedAt,
		Description: description,
		ChannelId:   channelId,
	}
	return &video
}

func (v *Video) CollectionName() string {
	return VideoCollection
}
