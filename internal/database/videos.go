package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/huandu/go-sqlbuilder"
	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type VideosClient interface {
	GetVideoByID(ctx context.Context, id string, o ...VideoQuery) (*models.Video, error)
	FindVideos(ctx context.Context, o ...VideoQuery) ([]*models.Video, error)
	UpsertVideos(ctx context.Context, videos ...*models.Video) error
}

type VideoQuery func(o *videoQuery)

type videoQuery struct {
	withChannel bool
	channels    []any
	typesIn     []any
	typesNotIn  []any
}

type videoQuerySlice []VideoQuery

func (s videoQuerySlice) opts() videoQuery {
	var q videoQuery
	for _, apply := range s {
		apply(&q)
	}
	return q
}

type Video struct{}

func (Video) WithChannel() VideoQuery {
	return func(o *videoQuery) {
		o.withChannel = true
	}
}
func (Video) Channel(id ...string) VideoQuery {
	return func(o *videoQuery) {
		o.channels = append(o.channels, toAny(id)...)
	}
}
func (Video) TypeEq(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesIn = append(o.typesIn, toAny(types)...)
	}
}
func (Video) TypeNot(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesNotIn = append(o.typesNotIn, toAny(types)...)
	}
}

func (c *sqliteClient) GetVideoByID(ctx context.Context, id string, o ...VideoQuery) (*models.Video, error) {
	opts := videoQuerySlice(o).opts()

	video, err := models.FindVideo(ctx, c.db, id)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := video.L.LoadChannel(ctx, c.db, true, video, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load video channel")
		}
	}

	return video, nil
}

func (c *sqliteClient) FindVideos(ctx context.Context, o ...VideoQuery) ([]*models.Video, error) {
	opts := videoQuerySlice(o).opts()

	sql := sqlbuilder.
		Select("*").
		From(models.TableNames.Videos)

	var where []string
	if opts.channels != nil {
		where = append(where, sql.In(models.VideoColumns.ChannelID, opts.channels...))
	}
	if opts.typesIn != nil {
		where = append(where, sql.In(models.VideoColumns.Type, opts.typesIn...))
	}
	if opts.typesNotIn != nil {
		where = append(where, sql.NotIn(models.VideoColumns.Type, opts.typesNotIn...))
	}
	if where != nil {
		sql = sql.Where(sql.And(where...))
	}

	var err error
	var videos []*models.Video
	q, a := sql.Build()
	videos, err = models.Videos(qm.SQL(q, a...)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := models.Video{}.L.LoadChannel(ctx, c.db, false, &videos, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load video channel")
		}
	}

	return videos, nil
}

func (c *sqliteClient) UpsertVideos(ctx context.Context, videos ...*models.Video) error {
	for _, v := range videos {
		err := v.Upsert(ctx, c.db, true, []string{models.VideoColumns.ID}, boil.Infer(), boil.Infer())
		if err != nil {
			return err
		}
	}
	return nil
}

type ViewsClient interface {
	GetUserViews(ctx context.Context, userID string, videoID ...string) ([]*models.View, error)
	UpsertView(ctx context.Context, view *models.View) error
}

func (c *sqliteClient) GetUserViews(ctx context.Context, userID string, videoID ...string) ([]*models.View, error) {
	sql := sqlbuilder.
		Select("*").
		From(models.TableNames.Views)

	where := []string{sql.EQ(models.ViewColumns.UserID, userID)}
	if videoID != nil {
		sql = sql.Where(sql.In(models.ViewColumns.VideoID, toAny(videoID)...))
	}
	sql = sql.Where(sql.And(where...))

	q, a := sql.Build()
	views, err := models.Views(qm.SQL(q, a...)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	return views, nil
}

func (c *sqliteClient) UpsertView(ctx context.Context, view *models.View) error {
	return view.Upsert(ctx, c.db, true, []string{models.ViewColumns.ID}, boil.Infer(), boil.Infer())
}
