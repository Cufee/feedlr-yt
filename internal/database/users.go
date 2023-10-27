package database

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/kamva/mgm/v3"
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
	return user, mgm.Coll(user).First(bson.M{"authId": authId}, user)
}

func (c *Client) NewUser(authId string) (*models.User, error) {
	user := models.NewUser(authId)
	return user, mgm.Coll(user).Create(user)
}
