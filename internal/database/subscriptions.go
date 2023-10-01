package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) NewSubscription(userId, channelId string) (*db.UserSubscriptionModel, error) {
	ul := db.UserSubscription.User.Link(db.User.ID.Equals(userId))
	cl := db.UserSubscription.Channel.Link(db.Channel.ID.Equals(channelId))
	return c.p.UserSubscription.CreateOne(ul, cl).With(db.UserSubscription.Channel.Fetch(), db.UserSubscription.User.Fetch()).Exec(context.TODO())
}

type SubscriptionGetOptions struct {
	WithChannel bool
	WithUser    bool
}

func (c *Client) AllUserSubscriptions(userId string, opts ...SubscriptionGetOptions) ([]db.UserSubscriptionModel, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	query := c.p.UserSubscription.FindMany(db.UserSubscription.UserID.Equals(userId))
	if options.WithChannel {
		query = query.With(db.UserSubscription.Channel.Fetch())
	}
	if options.WithUser {
		query = query.With(db.UserSubscription.User.Fetch())
	}

	return query.Exec(context.TODO())
}

func (c *Client) FindSubscription(userId, channelId string, opts ...SubscriptionGetOptions) (*db.UserSubscriptionModel, error) {
	var options SubscriptionGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	query := c.p.UserSubscription.FindFirst(db.UserSubscription.ChannelID.Equals(channelId), db.UserSubscription.UserID.Equals(userId))
	if options.WithChannel {
		query = query.With(db.UserSubscription.Channel.Fetch())
	}
	if options.WithUser {
		query = query.With(db.UserSubscription.User.Fetch())
	}

	return query.Exec(context.TODO())
}

func (c *Client) DeleteSubscription(userId, channelId string) error {
	sub, err := c.FindSubscription(userId, channelId)
	if err != nil {
		return err
	}
	_, err = c.p.UserSubscription.FindUnique(db.UserSubscription.ID.Equals(sub.ID)).Delete().Exec(context.TODO())
	return err
}
