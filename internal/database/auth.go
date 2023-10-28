package database

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
)

func (c *Client) NewAuthNonce(expiration time.Time, value string) (*models.AuthNonce, error) {
	nonce := models.NewAuthNonce(expiration, value)
	nonce.Prepare()

	ctx, cancel := c.Ctx()
	defer cancel()
	res, err := c.Collection(models.AuthNonceCollection).InsertOne(ctx, nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nonce.ParseID(res.InsertedID)
}

func (c *Client) FindNonce(value string) (*models.AuthNonce, error) {
	nonce := &models.AuthNonce{}
	ctx, cancel := c.Ctx()
	defer cancel()

	err := c.Collection(models.AuthNonceCollection).FindOne(ctx, bson.M{"value": value}).Decode(nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}
