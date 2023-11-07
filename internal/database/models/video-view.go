package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const VideoViewCollection = "video_views"

type VideoView struct {
	Model `bson:",inline"`

	User    *User              `json:"user" bson:"user,omitempty"`
	UserId  primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	Video   *Video             `json:"video" bson:"video,omitempty,omitempty"`
	VideoId string             `json:"videoId" bson:"videoId,omitempty"`

	Progress int `json:"progress" bson:"progress"`
}

func init() {
	addIndexHandler(VideoViewCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.M{"userId": 1},
			},
			{
				Keys: bson.M{"videoId": 1},
			},
			{
				Keys: bson.D{
					{Key: "userId", Value: 1},
					{Key: "videoId", Value: 1},
				},
			},
		})
	})
}

type VideoViewOptions struct {
	Progress *int
}

func NewVideoView(userId primitive.ObjectID, videoId string, opts ...VideoViewOptions) *VideoView {
	view := &VideoView{
		UserId:   userId,
		VideoId:  videoId,
		Progress: 0,
	}

	if len(opts) > 0 {
		if opts[0].Progress != nil {
			view.Progress = *opts[0].Progress
		}
	}

	return view
}

func (view *VideoView) CollectionName() string {
	return VideoViewCollection
}
