package models

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const AuthNonceCollection = "auth_nonces"

type AuthNonce struct {
	Model `bson:",inline"`

	ExpiresAt time.Time `json:"expiresAt" bson:"expiresAt"`
	Value     string    `json:"value" bson:"value"`
}

func init() {
	addIndexHandler(AuthNonceCollection, func(coll *mongo.Collection) ([]string, error) {
		expiration := int32((time.Hour * 24).Seconds())
		return coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: "expiresAt", Value: 1},
					{Key: "value", Value: 1},
				},
			},
			{
				Keys:    bson.M{"expiresAt": -1},
				Options: &options.IndexOptions{ExpireAfterSeconds: &expiration},
			},
			{
				Keys: bson.M{"value": 1},
			},
		})
	})
}

func NewAuthNonce(expiresAt time.Time, value string) *AuthNonce {
	return &AuthNonce{
		ExpiresAt: expiresAt,
		Value:     value,
	}
}

func (model *AuthNonce) CollectionName() string {
	return AuthNonceCollection
}
