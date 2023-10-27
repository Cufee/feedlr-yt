package database

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c *Client) EnsureUserExists(authId string) (*models.User, error) {
	user, err := c.GetUserFromAuthID(authId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return c.NewUser(authId)
		}
		return nil, err
	}
	return user, nil
}

func (c *Client) GetUserFromAuthID(authId string) (*models.User, error) {
	user := &models.User{}
	ctx, cancel := c.Ctx()
	defer cancel()

	return user, c.Collection(models.UserCollection).FindOne(ctx, bson.M{"authId": authId}).Decode(user)
}

func (c *Client) NewUser(authId string) (*models.User, error) {
	user := models.NewUser(authId)
	user.Prepare()

	ctx, cancel := c.Ctx()
	defer cancel()

	res, err := c.Collection(models.UserCollection).InsertOne(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, user.ParseID(res.InsertedID)
}
