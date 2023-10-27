package database

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

type Client struct{}

var DefaultClient *Client = NewClient()

func NewClient() *Client {
	connString, err := connstring.ParseAndValidate(utils.MustGetEnv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	uriOptions := options.Client().ApplyURI(connString.String())

	client, err := mongo.Connect(context.TODO(), uriOptions)
	if err != nil {
		panic(err)
	}
	defer client.Disconnect(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = client.Ping(ctx, nil)
	if err != nil {
		panic(err)
	}

	err = mgm.SetDefaultConfig(nil, connString.Database, uriOptions)
	if err != nil {
		panic(err)
	}

	return &Client{}
}
