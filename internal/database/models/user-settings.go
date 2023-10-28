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
	UserId primitive.ObjectID `json:"userId" bson:"userId" field:"required"`

	SponsorBlockEnabled    bool     `json:"sponsorBlockEnabled" bson:"sponsorBlockEnabled"`
	SponsorBlockCategories []string `json:"sponsorBlockCategories" bson:"sponsorBlockCategories"`
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
	SponsorBlockEnabled    *bool
	SponsorBlockCategories *[]string
}

func NewUserSettings(userId primitive.ObjectID, opts ...UserSettingsOptions) *UserSettings {
	settings := &UserSettings{
		UserId:                 userId,
		SponsorBlockEnabled:    true,
		SponsorBlockCategories: []string{},
	}

	if len(opts) > 0 {
		if opts[0].SponsorBlockEnabled != nil {
			settings.SponsorBlockEnabled = *opts[0].SponsorBlockEnabled
		}
		if opts[0].SponsorBlockCategories != nil {
			settings.SponsorBlockCategories = *opts[0].SponsorBlockCategories
		}
	}

	return settings
}

func (settings *UserSettings) CollectionName() string {
	return UserSettingsCollection
}
