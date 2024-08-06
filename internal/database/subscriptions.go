package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

type SubscriptionsClient interface {
	NewSubscription(ctx context.Context, userID string, channelID string) (*models.Subscription, error)
	FindSubscription(ctx context.Context, userID string, channelID string, opts ...SubscriptionQuery) (*models.Subscription, error)
	UserSubscriptions(ctx context.Context, userID string, o ...SubscriptionQuery) ([]*models.Subscription, error)
	GetSubscription(ctx context.Context, id string, opts ...SubscriptionQuery) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, sub *models.Subscription) error
	DeleteSubscription(ctx context.Context, userID string, channelID string) error
}

type subscriptionQuery struct {
	withChannel bool
	withUser    bool
}

type SubscriptionQuery func(*subscriptionQuery)

type subscriptionGetOptionSlice []SubscriptionQuery

func (s subscriptionGetOptionSlice) opts() subscriptionQuery {
	var o subscriptionQuery
	for _, apply := range s {
		apply(&o)
	}
	return o
}

type Subscription struct{}

func (Subscription) WithChannel() SubscriptionQuery {
	return func(o *subscriptionQuery) {
		o.withChannel = true
	}
}
func (Subscription) WithUser() SubscriptionQuery {
	return func(o *subscriptionQuery) {
		o.withUser = true
	}
}

func (c *sqliteClient) NewSubscription(ctx context.Context, userID, channelID string) (*models.Subscription, error) {
	sub := models.Subscription{
		UserID:    userID,
		ChannelID: channelID,
		Favorite:  false,
	}

	err := sub.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}

	return &sub, nil
}

func (c *sqliteClient) UserSubscriptions(ctx context.Context, userID string, o ...SubscriptionQuery) ([]*models.Subscription, error) {
	opts := subscriptionGetOptionSlice(o).opts()

	var err error
	var subscriptions []*models.Subscription // the type needs to be set for load functions to work
	subscriptions, err = models.Subscriptions(models.SubscriptionWhere.UserID.EQ(userID)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := models.Subscription{}.L.LoadChannel(ctx, c.db, false, &subscriptions, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription channel")
		}
	}
	if opts.withUser {
		err := models.Subscription{}.L.LoadUser(ctx, c.db, false, &subscriptions, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription user")
		}
	}

	return subscriptions, nil
}

func (c *sqliteClient) FindSubscription(ctx context.Context, userID, channelID string, o ...SubscriptionQuery) (*models.Subscription, error) {
	opts := subscriptionGetOptionSlice(o).opts()

	subscription, err := models.Subscriptions(models.SubscriptionWhere.UserID.EQ(userID), models.SubscriptionWhere.ChannelID.EQ(channelID)).One(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := models.Subscription{}.L.LoadChannel(ctx, c.db, true, subscription, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription channel")
		}
	}
	if opts.withUser {
		err := models.Subscription{}.L.LoadUser(ctx, c.db, true, subscription, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription user")
		}
	}

	return subscription, nil

}

func (c *sqliteClient) GetSubscription(ctx context.Context, id string, o ...SubscriptionQuery) (*models.Subscription, error) {
	opts := subscriptionGetOptionSlice(o).opts()

	subscription, err := models.FindSubscription(ctx, c.db, id)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := models.Subscription{}.L.LoadChannel(ctx, c.db, true, subscription, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription channel")
		}
	}
	if opts.withUser {
		err := models.Subscription{}.L.LoadUser(ctx, c.db, true, subscription, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load subscription user")
		}
	}

	return subscription, nil
}

func (c *sqliteClient) DeleteSubscription(ctx context.Context, userID string, channelID string) error {
	_, err := models.Subscriptions(models.SubscriptionWhere.UserID.EQ(userID), models.SubscriptionWhere.ChannelID.EQ(channelID)).DeleteAll(ctx, c.db)
	if err != nil {
		return err
	}
	return nil
}

func (c *sqliteClient) UpdateSubscription(ctx context.Context, sub *models.Subscription) error {
	_, err := sub.Update(ctx, c.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}
