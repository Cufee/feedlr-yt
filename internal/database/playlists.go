package database

import (
	"context"
	"database/sql"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/huandu/go-sqlbuilder"
	"github.com/pkg/errors"
)

type PlaylistsClient interface {
	GetPlaylistBySlug(ctx context.Context, userID, slug string) (*models.Playlist, error)
	GetPlaylistByID(ctx context.Context, playlistID string) (*models.Playlist, error)
	GetUserPlaylists(ctx context.Context, userID string) ([]*models.Playlist, error)
	CreatePlaylist(ctx context.Context, playlist *models.Playlist) error
	UpdatePlaylist(ctx context.Context, playlist *models.Playlist) error
	DeletePlaylist(ctx context.Context, playlistID string) error
	AddPlaylistItem(ctx context.Context, playlistID, videoID string) error
	AddPlaylistItemAtPosition(ctx context.Context, playlistID, videoID string, position int) error
	RemovePlaylistItem(ctx context.Context, playlistID, videoID string) error
	GetPlaylistItems(ctx context.Context, playlistID string, o ...PlaylistItemQuery) ([]*models.PlaylistItem, error)
	GetPlaylistItemCount(ctx context.Context, playlistID string) (int64, error)
	GetPlaylistFirstVideoID(ctx context.Context, playlistID string) (string, error)
	GetPlaylistItemByVideoID(ctx context.Context, playlistID, videoID string) (*models.PlaylistItem, error)
	SwapPlaylistItemPositions(ctx context.Context, playlistID, videoID, direction string) error
	IsVideoInPlaylist(ctx context.Context, playlistID, videoID string) (bool, error)
	CleanupExpiredPlaylistItems(ctx context.Context) (int64, error)
	GetMaxPlaylistItemPosition(ctx context.Context, playlistID string) (int, error)
}

// PlaylistItemQuery options
type PlaylistItemQuery func(o *playlistItemQuery)

type playlistItemQuery struct {
	limit           int
	offset          int
	withVideo       bool
	withChannel     bool
	orderByPosition bool
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

func (playlistItem) OrderByPosition() PlaylistItemQuery {
	return func(o *playlistItemQuery) {
		o.orderByPosition = true
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

func (c *sqliteClient) GetPlaylistByID(ctx context.Context, playlistID string) (*models.Playlist, error) {
	return models.FindPlaylist(ctx, c.db, playlistID)
}

func (c *sqliteClient) GetUserPlaylists(ctx context.Context, userID string) ([]*models.Playlist, error) {
	return models.Playlists(
		qm.Where(models.PlaylistColumns.UserID+"=?", userID),
		qm.Where(models.PlaylistColumns.System+"=?", false),
		qm.OrderBy(models.PlaylistColumns.UpdatedAt+" DESC"),
	).All(ctx, c.db)
}

func (c *sqliteClient) CreatePlaylist(ctx context.Context, playlist *models.Playlist) error {
	return playlist.Insert(ctx, c.db, boil.Infer())
}

func (c *sqliteClient) UpdatePlaylist(ctx context.Context, playlist *models.Playlist) error {
	_, err := playlist.Update(ctx, c.db, boil.Whitelist(
		models.PlaylistColumns.Name,
		models.PlaylistColumns.Description,
		models.PlaylistColumns.YoutubePlaylistID,
		models.PlaylistColumns.UpdatedAt,
	))
	return err
}

func (c *sqliteClient) DeletePlaylist(ctx context.Context, playlistID string) error {
	playlist, err := models.FindPlaylist(ctx, c.db, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to find playlist")
	}
	_, err = playlist.Delete(ctx, c.db)
	return err
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

	// Compute next position
	maxPos, err := c.GetMaxPlaylistItemPosition(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get max position")
	}

	item := &models.PlaylistItem{
		PlaylistID: playlistID,
		VideoID:    videoID,
		Position:   int64(maxPos + 1),
	}
	return item.Insert(ctx, c.db, boil.Infer())
}

func (c *sqliteClient) AddPlaylistItemAtPosition(ctx context.Context, playlistID, videoID string, position int) error {
	// Use INSERT OR IGNORE for idempotent imports
	_, err := c.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO playlist_items (id, created_at, updated_at, playlist_id, video_id, position) VALUES (?, datetime('now'), datetime('now'), ?, ?, ?)",
		ensureID(""), playlistID, videoID, position,
	)
	return err
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

	sb := sqlbuilder.
		Select("*").
		From(models.TableNames.PlaylistItems).
		Where(models.PlaylistItemColumns.PlaylistID + "=?")

	if opts.orderByPosition {
		sb = sb.OrderBy(models.PlaylistItemColumns.Position).Asc()
	} else {
		sb = sb.OrderBy(models.PlaylistItemColumns.CreatedAt).Desc()
	}

	if opts.limit > 0 {
		sb = sb.Limit(opts.limit)
	}
	if opts.offset > 0 {
		sb = sb.Offset(opts.offset)
	}

	q, _ := sb.Build()
	items, err := models.PlaylistItems(qm.SQL(q, playlistID)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	if opts.withVideo {
		itemsSlice := []*models.PlaylistItem(items)
		err := models.PlaylistItem{}.L.LoadVideo(ctx, c.db, false, &itemsSlice, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load videos")
		}

		if opts.withChannel {
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

func (c *sqliteClient) GetPlaylistItemCount(ctx context.Context, playlistID string) (int64, error) {
	return models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
	).Count(ctx, c.db)
}

func (c *sqliteClient) GetPlaylistFirstVideoID(ctx context.Context, playlistID string) (string, error) {
	item, err := models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		qm.OrderBy(models.PlaylistItemColumns.Position+" ASC"),
		qm.Limit(1),
	).One(ctx, c.db)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return item.VideoID, nil
}

func (c *sqliteClient) GetPlaylistItemByVideoID(ctx context.Context, playlistID, videoID string) (*models.PlaylistItem, error) {
	return models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		qm.Where(models.PlaylistItemColumns.VideoID+"=?", videoID),
	).One(ctx, c.db)
}

func (c *sqliteClient) GetMaxPlaylistItemPosition(ctx context.Context, playlistID string) (int, error) {
	var maxPos sql.NullInt64
	err := c.db.QueryRowContext(ctx,
		"SELECT MAX(position) FROM playlist_items WHERE playlist_id = ?",
		playlistID,
	).Scan(&maxPos)
	if err != nil {
		return 0, err
	}
	if !maxPos.Valid {
		return 0, nil
	}
	return int(maxPos.Int64), nil
}

func (c *sqliteClient) SwapPlaylistItemPositions(ctx context.Context, playlistID, videoID, direction string) error {
	// Find the item being moved
	item, err := c.GetPlaylistItemByVideoID(ctx, playlistID, videoID)
	if err != nil {
		return errors.Wrap(err, "failed to find playlist item")
	}

	// Find the neighbor to swap with
	var neighbor *models.PlaylistItem
	if direction == "up" {
		neighbor, err = models.PlaylistItems(
			qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
			qm.Where(models.PlaylistItemColumns.Position+"<?", item.Position),
			qm.OrderBy(models.PlaylistItemColumns.Position+" DESC"),
			qm.Limit(1),
		).One(ctx, c.db)
	} else {
		neighbor, err = models.PlaylistItems(
			qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
			qm.Where(models.PlaylistItemColumns.Position+">?", item.Position),
			qm.OrderBy(models.PlaylistItemColumns.Position+" ASC"),
			qm.Limit(1),
		).One(ctx, c.db)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Already at the edge, nothing to swap
		}
		return errors.Wrap(err, "failed to find neighbor item")
	}

	// Swap positions in a transaction
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Use a temporary position to avoid unique constraint issues if any
	_, err = tx.ExecContext(ctx, "UPDATE playlist_items SET position = ? WHERE id = ?", neighbor.Position, item.ID)
	if err != nil {
		return errors.Wrap(err, "failed to update item position")
	}
	_, err = tx.ExecContext(ctx, "UPDATE playlist_items SET position = ? WHERE id = ?", item.Position, neighbor.ID)
	if err != nil {
		return errors.Wrap(err, "failed to update neighbor position")
	}

	return tx.Commit()
}

func (c *sqliteClient) IsVideoInPlaylist(ctx context.Context, playlistID, videoID string) (bool, error) {
	exists, err := models.PlaylistItems(
		qm.Where(models.PlaylistItemColumns.PlaylistID+"=?", playlistID),
		qm.Where(models.PlaylistItemColumns.VideoID+"=?", videoID),
	).Exists(ctx, c.db)
	return exists, err
}

func (c *sqliteClient) CleanupExpiredPlaylistItems(ctx context.Context) (int64, error) {
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
