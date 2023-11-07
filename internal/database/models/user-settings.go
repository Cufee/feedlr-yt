package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const UserSettingsCollection = "user_settings"

type UserSettings struct {
	Model `bson:",inline"`

	User   *User              `json:"user" bson:"user,omitempty"`
	UserId primitive.ObjectID `json:"userId" bson:"userId,omitempty"`

	SponsorBlockEnabled    *bool    `json:"sponsorBlockEnabled" bson:"sponsorBlockEnabled,omitempty"`
	SponsorBlockCategories []string `json:"sponsorBlockCategories" bson:"sponsorBlockCategories,omitempty"`

	FeedMode string `json:"feedMode" bson:"feedMode,omitempty"`

	PlayerVolumeLevel int `json:"playerVolumeLevel" bson:"playerVolumeLevel,omitempty"`
}

func init() {
	addIndexHandler(UserSettingsCollection, func(coll *mongo.Collection) ([]string, error) {
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys:    bson.M{"userId": 1},
				Options: &options.IndexOptions{Unique: &[]bool{true}[0]},
			},
		})
	})
}

type UserSettingsOptions struct {
	FeedMode               *string
	PlayerVolumeLevel      *int
	SponsorBlockEnabled    *bool
	SponsorBlockCategories *[]string
}

func NewUserSettings(userId primitive.ObjectID, opts ...UserSettingsOptions) *UserSettings {
	settings := &UserSettings{
		UserId: userId,
	}

	if len(opts) > 0 {
		if opts[0].SponsorBlockEnabled != nil {
			settings.SponsorBlockEnabled = opts[0].SponsorBlockEnabled
		}
		if opts[0].SponsorBlockCategories != nil {
			settings.SponsorBlockCategories = *opts[0].SponsorBlockCategories
		}
		if opts[0].FeedMode != nil {
			settings.FeedMode = *opts[0].FeedMode
		}
		if opts[0].PlayerVolumeLevel != nil {
			settings.PlayerVolumeLevel = *opts[0].PlayerVolumeLevel
		}
	}

	return settings
}

func (settings *UserSettings) CollectionName() string {
	return UserSettingsCollection
}
