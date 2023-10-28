package database

import (
	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *Client) GetUserSettings(userId primitive.ObjectID) (*models.UserSettings, error) {
	settings := &models.UserSettings{}
	ctx, cancel := c.Ctx()
	defer cancel()

	return settings, c.Collection(models.UserSettingsCollection).FindOne(ctx, bson.M{"userId": userId}).Decode(settings)
}

func (c *Client) UpdateUserSettings(userId primitive.ObjectID, opts ...models.UserSettingsOptions) (*models.UserSettings, error) {
	settings := models.NewUserSettings(userId, opts...)
	settings.Prepare()

	ctx, cancel := c.Ctx()
	defer cancel()

	res, err := c.Collection(models.UserSettingsCollection).UpdateOne(ctx, bson.M{"userId": userId}, bson.M{"$set": settings}, options.Update().SetUpsert(true))
	if err != nil {
		return nil, err
	}
	if res.UpsertedID == nil {
		return settings, nil
	}
	return settings, settings.ParseID(res.UpsertedID)
}
