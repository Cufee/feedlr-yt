package database

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/doug-martin/goqu/v9"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
)

type ChannelsClient interface {
	GetChannel(ctx context.Context, channelId string, opts ...ChannelQuery) (*models.Channel, error)
	GetChannels(ctx context.Context, opts ...ChannelQuery) ([]*models.Channel, error)
	GetChannelsForUpdate(ctx context.Context) ([]string, error)
	UpsertChannel(ctx context.Context, data *models.Channel) error
	SetChannelFeedUpdatedAt(ctx context.Context, channelID string, updatedAt time.Time) error
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

		if len(ids) > 0 {
			sql := goqu.From(models.TableNames.Videos).
				Select(goqu.Star()).
				Order(
					goqu.I(models.VideoColumns.PublishedAt).Desc(),
					goqu.I(models.VideoColumns.CreatedAt).Desc(),
					goqu.I(models.VideoColumns.ID).Desc(),
				).
				Where(
					goqu.I(models.VideoColumns.ChannelID).In(ids...),
					goqu.I(models.VideoColumns.Type).NotIn("private", "short"),
				)
			if opts.videosLimit > 0 {
				sql = sql.Limit(uint(opts.videosLimit))
			}

			q, a, err := sql.ToSQL()
			if err != nil {
				return nil, err
			}
			err = models.Channel{}.L.LoadVideos(ctx, c.db, false, &channels, qm.SQL(q, a...))
			if err != nil {
				return nil, err
			}
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

func (c *sqliteClient) GetChannelsForUpdate(ctx context.Context) ([]string, error) {
	subscriptions, err := models.Subscriptions(qm.GroupBy(models.SubscriptionColumns.ChannelID), qm.Select(models.SubscriptionColumns.ChannelID)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	var channelIDs []string
	for _, s := range subscriptions {
		channelIDs = append(channelIDs, s.ChannelID)
	}
	if len(channelIDs) == 0 {
		return nil, nil
	}

	const maxVideosPerChannel uint = 5

	// Define a window function to assign a row number partitioned by ChannelID and ordered by PublishedAt
	rowNumber := goqu.L("ROW_NUMBER() OVER (PARTITION BY ? ORDER BY ? DESC)", goqu.I(models.VideoColumns.ChannelID), goqu.I(models.VideoColumns.PublishedAt))

	// Subquery to get row numbers for each video
	subQuery := goqu.From(models.TableNames.Videos).
		Select(
			models.VideoColumns.ChannelID,
			models.VideoColumns.PublishedAt,
			rowNumber.As("row_num"),
		).
		Where(goqu.I(models.VideoColumns.ChannelID).In(toAny(channelIDs)...))

	// Main query to filter out only the last 5 videos per ChannelID
	query, args, err := goqu.From(subQuery.As("sub")).
		Select(models.VideoColumns.ChannelID, models.VideoColumns.PublishedAt).
		Where(goqu.I("row_num").Lte(maxVideosPerChannel)).
		Order(goqu.I(models.VideoColumns.PublishedAt).Desc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	videos, err := models.Videos(qm.SQL(query, args...)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	channelUploads := make(map[string][]time.Time)
	for _, r := range videos {
		channelUploads[r.ChannelID] = append(channelUploads[r.ChannelID], r.PublishedAt)
	}

	var toUpdate []string
	for _, id := range channelIDs {
		uploads := channelUploads[id]
		if len(uploads) < 3 {
			toUpdate = append(toUpdate, id)
			continue
		}

		// Calculate the average frequency of uploads
		var total float64
		for i := 0; i < len(uploads)-1; i++ {
			total += uploads[i].Sub(uploads[i+1]).Seconds()
		}
		averageFreq := total / float64(len(uploads)-1)

		// Determine if the channel should be updated
		if time.Since(uploads[0]).Seconds() < (averageFreq * 0.5) {
			continue
		}
		toUpdate = append(toUpdate, id)
	}
	return toUpdate, nil
}

func (c *sqliteClient) GetChannel(ctx context.Context, channelId string, o ...ChannelQuery) (*models.Channel, error) {
	opts := channelQuerySlice(o).opts()

	channel, err := models.FindChannel(ctx, c.db, channelId)
	if err != nil {
		return nil, err
	}

	if opts.withVideos {
		sql := goqu.From(models.TableNames.Videos).
			Select(goqu.Star()).
			Order(
				goqu.I(models.VideoColumns.PublishedAt).Desc(),
				goqu.I(models.VideoColumns.CreatedAt).Desc(),
				goqu.I(models.VideoColumns.ID).Desc(),
			)
		sql = sql.Where(
			goqu.I(models.VideoColumns.ChannelID).Eq(channel.ID),
			goqu.I(models.VideoColumns.Type).NotIn("private", "short"),
		)
		if opts.videosLimit > 0 {
			sql = sql.Limit(uint(opts.videosLimit))
		}

		q, a, err := sql.ToSQL()
		if err != nil {
			return nil, err
		}
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

func (c *sqliteClient) SetChannelFeedUpdatedAt(ctx context.Context, channelID string, updatedAt time.Time) error {
	_, err := models.Channels(models.ChannelWhere.ID.EQ(channelID)).UpdateAll(ctx, c.db, models.M{models.ChannelColumns.FeedUpdatedAt: updatedAt})
	return err
}
