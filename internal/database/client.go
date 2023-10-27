package database

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/utils"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

type Client struct {
	db *mongo.Database
}

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = client.Ping(ctx, nil)
	if err != nil {
		panic(err)
	}

	return &Client{
		db: client.Database(connString.Database),
	}
}

func (c *Client) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	c.db.Client().Disconnect(ctx)
}

func (c *Client) Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*5)
}

func (c *Client) Collection(coll string) *mongo.Collection {
	return c.db.Collection(coll)
}
