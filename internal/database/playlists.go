package database

import (
	"context"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/huandu/go-sqlbuilder"
	"github.com/pkg/errors"
)

type PlaylistsClient interface {
	GetPlaylistBySlug(ctx context.Context, userID, slug string) (*models.Playlist, error)
	CreatePlaylist(ctx context.Context, playlist *models.Playlist) error
	AddPlaylistItem(ctx context.Context, playlistID, videoID string) error
	RemovePlaylistItem(ctx context.Context, playlistID, videoID string) error
	GetPlaylistItems(ctx context.Context, playlistID string, o ...PlaylistItemQuery) ([]*models.PlaylistItem, error)
	IsVideoInPlaylist(ctx context.Context, playlistID, videoID string) (bool, error)
	CleanupExpiredPlaylistItems(ctx context.Context) (int64, error)
}


// PlaylistItemQuery options
type PlaylistItemQuery func(o *playlistItemQuery)

type playlistItemQuery struct {
	limit       int
	offset      int
	withVideo   bool
	withChannel bool
}

type playlistItemQuerySlice []PlaylistItemQuery

func (s playlistItemQuerySlice) opts() playlistItemQuery {
	var q playlistItemQuery
	for _, apply := range s {
		apply(&q)
	}
	return q
}

var PlaylistItem playlistItem

type playlistItem struct{}

func (playlistItem) Limit(n int) PlaylistItemQuery {
	return func(o *playlistItemQuery) {
		o.limit = n
	}
}

func (playlistItem) Offset(n int) PlaylistItemQuery {
	return func(o *playlistItemQuery) {
		o.offset = n
	}
}

func (playlistItem) WithVideo() PlaylistItemQuery {
	return func(o *playlistItemQuery) {
		o.withVideo = true
	}
}

func (playlistItem) WithChannel() PlaylistItemQuery {
	return func(o *playlistItemQuery) {
		o.withChannel = true
	}
}

func (c *sqliteClient) GetPlaylistBySlug(ctx context.Context, userID, slug string) (*models.Playlist, error) {
	playlist, err := models.Playlists(
		qm.Where(models.PlaylistColumns.UserID+"=?", userID),
		qm.Where(models.PlaylistColumns.Slug+"=?", slug),
	).One(ctx, c.db)
	if err != nil {
		return nil, err
	}

	return playlist, nil
}

func (c *sqliteClient) CreatePlaylist(ctx context.Context, playlist *models.Playlist) error {
	return playlist.Insert(ctx, c.db, boil.Infer())
}

func (c *sqliteClient) AddPlaylistItem(ctx context.Context, playlistID, videoID string) error {
	// First, check max_size constraint
	playlist, err := models.FindPlaylist(ctx, c.db, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to find playlist")
	}

	if playlist.MaxSize.Valid && playlist.MaxSize.Int64 > 0 {
		// Count current items
		count, err := models.PlaylistItems(
			qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		).Count(ctx, c.db)
		if err != nil {
			return errors.Wrap(err, "failed to count playlist items")
		}

		// If at max size, remove oldest items
		if count >= playlist.MaxSize.Int64 {
			toRemove := count - playlist.MaxSize.Int64 + 1
			oldestItems, err := models.PlaylistItems(
				qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
				qm.OrderBy(models.PlaylistItemColumns.CreatedAt+" ASC"),
				qm.Limit(int(toRemove)),
			).All(ctx, c.db)
			if err != nil {
				return errors.Wrap(err, "failed to find oldest items")
			}
			for _, item := range oldestItems {
				_, err := item.Delete(ctx, c.db)
				if err != nil {
					return errors.Wrap(err, "failed to delete oldest item")
				}
			}
		}
	}

	// Add the new item
	item := &models.PlaylistItem{
		PlaylistID: playlistID,
		VideoID:    videoID,
		Position:   0,
	}
	return item.Insert(ctx, c.db, boil.Infer())
}

func (c *sqliteClient) RemovePlaylistItem(ctx context.Context, playlistID, videoID string) error {
	_, err := models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		qm.Where(models.PlaylistItemColumns.VideoID+"=?", videoID),
	).DeleteAll(ctx, c.db)
	return err
}

func (c *sqliteClient) GetPlaylistItems(ctx context.Context, playlistID string, o ...PlaylistItemQuery) ([]*models.PlaylistItem, error) {
	opts := playlistItemQuerySlice(o).opts()

	sql := sqlbuilder.
		Select("*").
		From(models.TableNames.PlaylistItems).
		Where(models.PlaylistItemColumns.PlaylistID + "=?").
		OrderBy(models.PlaylistItemColumns.CreatedAt).Desc()

	if opts.limit > 0 {
		sql = sql.Limit(opts.limit)
	}
	if opts.offset > 0 {
		sql = sql.Offset(opts.offset)
	}

	q, _ := sql.Build()
	items, err := models.PlaylistItems(qm.SQL(q, playlistID)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withVideo {
		// Convert to the expected pointer type for the loader
		itemsSlice := []*models.PlaylistItem(items)
		err := models.PlaylistItem{}.L.LoadVideo(ctx, c.db, false, &itemsSlice, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load videos")
		}

		if opts.withChannel {
			// Load channels for each video
			var videos []*models.Video
			for _, item := range items {
				if item.R != nil && item.R.Video != nil {
					videos = append(videos, item.R.Video)
				}
			}
			if len(videos) > 0 {
				err := models.Video{}.L.LoadChannel(ctx, c.db, false, &videos, nil)
				if err != nil {
					return nil, errors.Wrap(err, "failed to load channels")
				}
			}
		}
	}

	return items, nil
}

func (c *sqliteClient) IsVideoInPlaylist(ctx context.Context, playlistID, videoID string) (bool, error) {
	exists, err := models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		qm.Where(models.PlaylistItemColumns.VideoID+"=?", videoID),
	).Exists(ctx, c.db)
	return exists, err
}

func (c *sqliteClient) CleanupExpiredPlaylistItems(ctx context.Context) (int64, error) {
	// Get all playlists with TTL
	playlists, err := models.Playlists(
		qm.Where(models.PlaylistColumns.TTLDays+" IS NOT NULL"),
	).All(ctx, c.db)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get playlists with TTL")
	}

	var totalDeleted int64
	for _, playlist := range playlists {
		if !playlist.TTLDays.Valid {
			continue
		}

		// Delete items older than TTL
		// SQLite: date('now', '-X days')
		result, err := c.db.ExecContext(ctx,
			"DELETE FROM playlist_items WHERE playlist_id = ? AND created_at < date('now', '-' || ? || ' days')",
			playlist.ID, playlist.TTLDays.Int64,
		)
		if err != nil {
			return totalDeleted, errors.Wrap(err, "failed to delete expired items")
		}
		deleted, _ := result.RowsAffected()
		totalDeleted += deleted
	}

	return totalDeleted, nil
}

// Helper function to create a system playlist with defaults
func NewWatchLaterPlaylist(userID string) *models.Playlist {
	return &models.Playlist{
		UserID:  userID,
		Slug:    "watch-later",
		Name:    "Watch Later",
		System:  true,
		TTLDays: null.Int64From(30),
	}
}
