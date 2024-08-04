package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
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
	channels    []string
	typesIn     []string
	typesNotIn  []string
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
		o.channels = append(o.channels, id...)
	}
}
func (Video) TypeEq(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesIn = append(o.typesIn, types...)
	}
}
func (Video) TypeNot(types ...string) VideoQuery {
	return func(o *videoQuery) {
		o.typesNotIn = append(o.typesNotIn, types...)
	}
}

func (c *sqliteClient) GetVideoByID(ctx context.Context, id string, o ...VideoQuery) (*models.Video, error) {
	opts := videoQuerySlice(o).opts()

	video, err := models.FindVideo(ctx, c.db, id)
	if err != nil {
		return nil, err
	}

	if opts.withChannel {
		err := video.L.LoadChannel(ctx, c.db, true, &video, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load video channel")
		}
	}

	return video, nil
}

func (c *sqliteClient) FindVideos(ctx context.Context, o ...VideoQuery) ([]*models.Video, error) {
	opts := videoQuerySlice(o).opts()

	mods := []qm.QueryMod{}
	if opts.channels != nil {
		mods = append(mods, models.VideoWhere.ChannelID.IN(opts.channels))
	}
	if opts.typesIn != nil {
		mods = append(mods, models.VideoWhere.Type.IN(opts.typesIn))
	}
	if opts.typesNotIn != nil {
		mods = append(mods, models.VideoWhere.Type.NIN(opts.typesNotIn))
	}

	videos, err := models.Videos(mods...).All(ctx, c.db)
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
	GetVideoByID(ctx context.Context, id string, o ...VideoQuery) (*models.Video, error)
	FindVideos(ctx context.Context, o ...VideoQuery) ([]*models.Video, error)
}

func (c *sqliteClient) GetUserViews(ctx context.Context, userID string, videoID ...string) ([]*models.View, error) {
	mods := []qm.QueryMod{models.ViewWhere.UserID.EQ(userID)}
	if videoID != nil {
		mods = append(mods, models.ViewWhere.VideoID.IN(videoID))
	}

	views, err := models.Views(mods...).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	return views, nil
}

func (c *sqliteClient) UpsertView(ctx context.Context, view *models.View) error {
	return view.Upsert(ctx, c.db, true, []string{models.ViewColumns.ID}, boil.Infer(), boil.Infer())
}
