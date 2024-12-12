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
	limit       int
	id          []any
	channels    []any
	typesIn     []any
	typesNotIn  []any
	columns     []string
}

type videoQuerySlice []VideoQuery

func (s videoQuerySlice) opts() videoQuery {
	var q videoQuery
	for _, apply := range s {
		apply(&q)
	}
	return q
}

var Video video

type video struct{}

func (video) WithChannel() VideoQuery {
	return func(o *videoQuery) {
		o.withChannel = true
	}
}
func (video) Channel(id ...string) VideoQuery {
	return func(o *videoQuery) {
		o.channels = append(o.channels, toAny(id)...)
	}
}
func (video) TypeEq(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesIn = append(o.typesIn, toAny(types)...)
	}
}
func (video) TypeNot(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesNotIn = append(o.typesNotIn, toAny(types)...)
	}
}
func (video) ID(id ...string) VideoQuery {
	return func(o *videoQuery) {
		o.id = append(o.id, toAny(id)...)
	}
}
func (video) Limit(limit int) VideoQuery {
	return func(o *videoQuery) {
		o.limit = limit
	}
}
func (video) Select(columns ...string) VideoQuery {
	return func(o *videoQuery) {
		o.columns = append(o.columns, columns...)
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

	withSelect := sqlbuilder.Select("*")
	if opts.columns != nil {
		withSelect = sqlbuilder.Select(opts.columns...)
	}

	sql := withSelect.
		From(models.TableNames.Videos).
		OrderBy(models.ViewColumns.CreatedAt).Desc()
	if opts.limit > 0 {
		sql = sql.Limit(opts.limit)
	}

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
	if opts.id != nil {
		where = append(where, sql.In(models.VideoColumns.ID, opts.id...))
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
		err := v.Upsert(ctx, c.db, true, []string{models.VideoColumns.ID}, boil.Blacklist(models.VideoColumns.CreatedAt), boil.Infer())
		if err != nil {
			return err
		}
	}
	return nil
}

type ViewsClient interface {
	GetUserViews(ctx context.Context, userID string, videoID ...string) ([]*models.View, error)
	GetRecentUserViews(ctx context.Context, userID string, limit int) ([]*models.View, error)
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
	sql = sql.Desc().OrderBy(models.ViewColumns.UpdatedAt)

	q, a := sql.Build()
	views, err := models.Views(qm.SQL(q, a...)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	return views, nil
}

func (c *sqliteClient) GetRecentUserViews(ctx context.Context, userID string, limit int) ([]*models.View, error) {
	sql := sqlbuilder.
		Select("*").
		From(models.TableNames.Views)

	sql = sql.Where(sql.EQ(models.ViewColumns.UserID, userID))
	sql = sql.Desc().OrderBy(models.ViewColumns.UpdatedAt)
	if limit > 0 {
		sql = sql.Limit(limit)
	}

	q, a := sql.Build()
	views, err := models.Views(qm.SQL(q, a...)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	return views, nil
}

func (c *sqliteClient) UpsertView(ctx context.Context, view *models.View) error {
	views, err := c.GetUserViews(ctx, view.UserID, view.VideoID)
	if IsErrNotFound(err) || len(views) == 0 {
		return view.Insert(ctx, c.db, boil.Infer())
	}
	if err != nil {
		return err
	}

	for _, v := range views {
		v.Hidden = view.Hidden
		v.Progress = view.Progress
		_, err := v.Update(ctx, c.db, boil.Whitelist(models.ViewColumns.Progress, models.ViewColumns.UpdatedAt, models.ViewColumns.Hidden))
		if err != nil {
			return err
		}
	}

	return nil
}
