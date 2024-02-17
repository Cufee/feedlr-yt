package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const PlaylistCollection = "playlists"

type Playlist struct {
	Model `bson:",inline"`

	Name string `json:"name" bson:"name,omitempty"`

	InternalUsers  []User             `json:"users" bson:"users,omitempty"`
	UserId         primitive.ObjectID `json:"userId" bson:"userId,omitempty"`
	InternalVideos []Video            `json:"videos" bson:"videos,omitempty"`
	VideoIds       []string           `json:"videoIds" bson:"videoIds,omitempty"`
}

func init() {
	addIndexHandler(PlaylistCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.M{"userId": 1},
			},
		})
	})
}

func (model *Playlist) User() *User {
	if len(model.InternalUsers) > 0 {
		return &model.InternalUsers[0]
	}
	return nil
}

func (model *Playlist) Videos() []Video {
	return model.InternalVideos
}

type PlaylistOptions struct {
}

func NewPlaylist(userId primitive.ObjectID, name string, videoIds []string, opts ...PlaylistOptions) *Playlist {
	playlist := &Playlist{
		Name:     name,
		UserId:   userId,
		VideoIds: videoIds,
	}

	return playlist
}

func (playlist *Playlist) CollectionName() string {
	return PlaylistCollection
}
