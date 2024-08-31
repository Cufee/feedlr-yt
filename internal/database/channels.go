package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/huandu/go-sqlbuilder"

	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type ChannelsClient interface {
	GetChannel(ctx context.Context, channelId string, opts ...ChannelQuery) (*models.Channel, error)
	GetChannels(ctx context.Context, opts ...ChannelQuery) ([]*models.Channel, error)
	GetChannelsWithSubscriptions(ctx context.Context) ([]*models.Channel, error)
	UpsertChannel(ctx context.Context, data *models.Channel) error
}

type ChannelQuery func(*channelQuery)

type channelQuery struct {
	id                []string
	withVideos        bool
	videosLimit       int
	withSubscriptions bool
}

type channelQuerySlice []ChannelQuery

func (s channelQuerySlice) opts() channelQuery {
	var o channelQuery
	for _, apply := range s {
		apply(&o)
	}
	return o
}

var Channel channel

type channel struct{}

func (channel) WithVideos(limit int) ChannelQuery {
	return func(o *channelQuery) {
		o.videosLimit = limit
		o.withVideos = true
	}
}
func (channel) WithSubscriptions() ChannelQuery {
	return func(o *channelQuery) {
		o.withSubscriptions = true
	}
}
func (channel) ID(ids ...string) ChannelQuery {
	return func(o *channelQuery) {
		o.id = append(o.id, ids...)
	}
}

func (c *sqliteClient) GetChannels(ctx context.Context, o ...ChannelQuery) ([]*models.Channel, error) {
	opts := channelQuerySlice(o).opts()

	var mods qm.QueryMod
	if opts.id != nil {
		mods = models.ChannelWhere.ID.IN(opts.id)
	}

	var err error
	var channels []*models.Channel
	channels, err = models.Channels(mods).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withVideos {
		var ids []any
		for _, c := range channels {
			ids = append(ids, c.ID)
		}

		sql := sqlbuilder.
			Select("*").
			From(models.TableNames.Videos).
			Desc().OrderBy(models.VideoColumns.PublishedAt)
		sql = sql.Where(
			sql.And(
				sql.In(models.VideoColumns.ChannelID, ids...),
				sql.NotIn(models.VideoColumns.Type, "private", "short"),
			),
		)
		if opts.videosLimit > 0 {
			sql = sql.Limit(opts.videosLimit)
		}

		q, a := sql.Build()
		err = models.Channel{}.L.LoadVideos(ctx, c.db, false, &channels, qm.SQL(q, a...))
		if err != nil {
			return nil, err
		}

	}
	if opts.withSubscriptions {
		err = models.Channel{}.L.LoadSubscriptions(ctx, c.db, false, &channels, nil)
		if err != nil {
			return nil, err
		}
	}

	return channels, nil
}

func (c *sqliteClient) GetChannelsWithSubscriptions(ctx context.Context) ([]*models.Channel, error) {
	var subscriptions []struct {
		ChannelID string `boil:"channel_id"`
	}

	err := models.Subscriptions(qm.GroupBy(models.SubscriptionColumns.ChannelID), qm.Select(models.SubscriptionColumns.ChannelID)).Bind(ctx, c.db, &subscriptions)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, s := range subscriptions {
		ids = append(ids, s.ChannelID)
	}

	channels, err := models.Channels(models.ChannelWhere.ID.IN(ids)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *sqliteClient) GetChannel(ctx context.Context, channelId string, o ...ChannelQuery) (*models.Channel, error) {
	opts := channelQuerySlice(o).opts()

	channel, err := models.FindChannel(ctx, c.db, channelId)
	if err != nil {
		return nil, err
	}

	if opts.withVideos {
		sql := sqlbuilder.
			Select("*").
			From(models.TableNames.Videos).
			Desc().OrderBy(models.VideoColumns.PublishedAt)
		sql = sql.Where(
			sql.And(
				sql.EQ(models.VideoColumns.ChannelID, channel.ID),
				sql.NotIn(models.VideoColumns.Type, "private", "short"),
			),
		)
		if opts.videosLimit > 0 {
			sql = sql.Limit(opts.videosLimit)
		}

		q, a := sql.Build()
		err = models.Channel{}.L.LoadVideos(ctx, c.db, true, channel, qm.SQL(q, a...))
		if err != nil {
			return nil, err
		}
	}
	if opts.withSubscriptions {
		err = models.Channel{}.L.LoadSubscriptions(ctx, c.db, true, channel, nil)
		if err != nil {
			return nil, err
		}
	}

	return channel, nil
}

func (c *sqliteClient) UpsertChannel(ctx context.Context, data *models.Channel) error {
	return data.Upsert(ctx, c.db, true, []string{models.ChannelColumns.ID}, boil.Infer(), boil.Infer())
}
