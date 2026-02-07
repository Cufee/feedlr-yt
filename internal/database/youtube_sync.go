package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/pkg/errors"
)

type YouTubeSyncClient interface {
	GetYouTubeSyncAccountByUserID(ctx context.Context, userID string) (*models.YoutubeSyncAccount, error)
	UpsertYouTubeSyncCredentials(ctx context.Context, userID string, encryptedRefreshToken []byte, secretHash string) error
	SetYouTubeSyncAccountEnabled(ctx context.Context, userID string, enabled bool) error
	DeleteYouTubeSyncAccount(ctx context.Context, userID string) error
	ListEnabledYouTubeSyncAccounts(ctx context.Context, limit int) ([]*models.YoutubeSyncAccount, error)
	UpdateYouTubeSyncPlaylistID(ctx context.Context, userID, playlistID string) error
	UpdateYouTubeSyncRunResult(ctx context.Context, userID string, result YouTubeSyncRunResult) error
}

type YouTubeSyncRunResult struct {
	LastFeedVideoPublishedAt null.Time
	LastSyncedAt             null.Time
	LastSyncAttemptAt        time.Time
	LastError                string
}

func (c *sqliteClient) GetYouTubeSyncAccountByUserID(ctx context.Context, userID string) (*models.YoutubeSyncAccount, error) {
	account, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.UserID.EQ(userID),
	).One(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (c *sqliteClient) UpsertYouTubeSyncCredentials(ctx context.Context, userID string, encryptedRefreshToken []byte, secretHash string) error {
	account := &models.YoutubeSyncAccount{
		UserID:          userID,
		RefreshTokenEnc: encryptedRefreshToken,
		EncSecretHash:   secretHash,
		SyncEnabled:     true,
		LastError:       "",
	}

	return account.Upsert(
		ctx,
		c.db,
		true,
		[]string{models.YoutubeSyncAccountColumns.UserID},
		boil.Whitelist(
			models.YoutubeSyncAccountColumns.RefreshTokenEnc,
			models.YoutubeSyncAccountColumns.EncSecretHash,
			models.YoutubeSyncAccountColumns.SyncEnabled,
			models.YoutubeSyncAccountColumns.LastError,
			models.YoutubeSyncAccountColumns.UpdatedAt,
		),
		boil.Infer(),
	)
}

func (c *sqliteClient) SetYouTubeSyncAccountEnabled(ctx context.Context, userID string, enabled bool) error {
	updated, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.UserID.EQ(userID),
	).UpdateAll(ctx, c.db, models.M{
		models.YoutubeSyncAccountColumns.SyncEnabled: enabled,
	})
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) DeleteYouTubeSyncAccount(ctx context.Context, userID string) error {
	deleted, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.UserID.EQ(userID),
	).DeleteAll(ctx, c.db)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) ListEnabledYouTubeSyncAccounts(ctx context.Context, limit int) ([]*models.YoutubeSyncAccount, error) {
	if limit <= 0 {
		limit = 100
	}

	accounts, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.SyncEnabled.EQ(true),
		qm.OrderBy(models.YoutubeSyncAccountColumns.LastSyncedAt+" ASC"),
		qm.OrderBy(models.YoutubeSyncAccountColumns.UpdatedAt+" ASC"),
		qm.Limit(limit),
	).All(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (c *sqliteClient) UpdateYouTubeSyncPlaylistID(ctx context.Context, userID, playlistID string) error {
	updated, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.UserID.EQ(userID),
	).UpdateAll(ctx, c.db, models.M{
		models.YoutubeSyncAccountColumns.PlaylistID: null.StringFrom(playlistID),
	})
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) UpdateYouTubeSyncRunResult(ctx context.Context, userID string, result YouTubeSyncRunResult) error {
	if result.LastSyncAttemptAt.IsZero() {
		return errors.New("last sync attempt timestamp is required")
	}

	updated, err := models.YoutubeSyncAccounts(
		models.YoutubeSyncAccountWhere.UserID.EQ(userID),
	).UpdateAll(ctx, c.db, models.M{
		models.YoutubeSyncAccountColumns.LastFeedVideoPublishedAt: result.LastFeedVideoPublishedAt,
		models.YoutubeSyncAccountColumns.LastSyncedAt:             result.LastSyncedAt,
		models.YoutubeSyncAccountColumns.LastSyncAttemptAt:        null.TimeFrom(result.LastSyncAttemptAt.UTC()),
		models.YoutubeSyncAccountColumns.LastError:                result.LastError,
	})
	if err != nil {
		return err
	}
	if updated == 0 {
		return sql.ErrNoRows
	}
	return nil
}
