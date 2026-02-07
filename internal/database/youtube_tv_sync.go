package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/lucsky/cuid"
)

const (
	youTubeTVSyncStateDisconnected = "disconnected"
)

type YouTubeTVSyncAccount struct {
	ID               string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	UserID           string
	ScreenID         string
	ScreenName       string
	LoungeTokenEnc   []byte
	EncSecretHash    string
	SyncEnabled      bool
	ConnectionState  string
	StateReason      string
	LastConnectedAt  null.Time
	LastEventAt      null.Time
	LastDisconnectAt null.Time
	LastUserActivity null.Time
	LastVideoID      null.String
	LastError        string
}

type YouTubeTVSyncStateUpdate struct {
	ConnectionState  string
	StateReason      string
	LastError        string
	LastConnectedAt  null.Time
	LastEventAt      null.Time
	LastDisconnectAt null.Time
	LastUserActivity null.Time
	LastVideoID      null.String
}

type YouTubeTVSyncClient interface {
	GetYouTubeTVSyncAccountByUserID(ctx context.Context, userID string) (*YouTubeTVSyncAccount, error)
	UpsertYouTubeTVSyncCredentials(ctx context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error
	UpdateYouTubeTVSyncLoungeToken(ctx context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error
	SetYouTubeTVSyncAccountEnabled(ctx context.Context, userID string, enabled bool) error
	DeleteYouTubeTVSyncAccount(ctx context.Context, userID string) error
	ListEnabledYouTubeTVSyncAccounts(ctx context.Context, limit int) ([]*YouTubeTVSyncAccount, error)
	UpdateYouTubeTVSyncState(ctx context.Context, userID string, update YouTubeTVSyncStateUpdate) error
	GetUserLastSessionActivity(ctx context.Context, userID string) (null.Time, error)
}

func scanYouTubeTVSyncAccount(row scanner) (*YouTubeTVSyncAccount, error) {
	account := &YouTubeTVSyncAccount{}
	var lastConnected sql.NullTime
	var lastEvent sql.NullTime
	var lastDisconnect sql.NullTime
	var lastUserActivity sql.NullTime
	var lastVideoID sql.NullString

	err := row.Scan(
		&account.ID,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.UserID,
		&account.ScreenID,
		&account.ScreenName,
		&account.LoungeTokenEnc,
		&account.EncSecretHash,
		&account.SyncEnabled,
		&account.ConnectionState,
		&account.StateReason,
		&lastConnected,
		&lastEvent,
		&lastDisconnect,
		&lastUserActivity,
		&lastVideoID,
		&account.LastError,
	)
	if err != nil {
		return nil, err
	}

	account.LastConnectedAt = null.TimeFromPtr(timePtrFromNull(lastConnected))
	account.LastEventAt = null.TimeFromPtr(timePtrFromNull(lastEvent))
	account.LastDisconnectAt = null.TimeFromPtr(timePtrFromNull(lastDisconnect))
	account.LastUserActivity = null.TimeFromPtr(timePtrFromNull(lastUserActivity))
	account.LastVideoID = null.StringFromPtr(stringPtrFromNull(lastVideoID))
	return account, nil
}

func (c *sqliteClient) GetYouTubeTVSyncAccountByUserID(ctx context.Context, userID string) (*YouTubeTVSyncAccount, error) {
	row := c.db.QueryRowContext(
		ctx,
		`SELECT id, created_at, updated_at, user_id, screen_id, screen_name, lounge_token_enc, enc_secret_hash, sync_enabled, connection_state, state_reason, last_connected_at, last_event_at, last_disconnect_at, last_user_activity_at, last_video_id, last_error
         FROM youtube_tv_sync_accounts
         WHERE user_id = ?`,
		userID,
	)

	account, err := scanYouTubeTVSyncAccount(row)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (c *sqliteClient) UpsertYouTubeTVSyncCredentials(ctx context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error {
	now := time.Now().UTC()

	_, err := c.db.ExecContext(
		ctx,
		`INSERT INTO youtube_tv_sync_accounts
        (id, created_at, updated_at, user_id, screen_id, screen_name, lounge_token_enc, enc_secret_hash, sync_enabled, connection_state, state_reason, last_connected_at, last_event_at, last_disconnect_at, last_user_activity_at, last_video_id, last_error)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, NULL, '')
         ON CONFLICT(user_id) DO UPDATE SET
            updated_at = excluded.updated_at,
            screen_id = excluded.screen_id,
            screen_name = excluded.screen_name,
            lounge_token_enc = excluded.lounge_token_enc,
            enc_secret_hash = excluded.enc_secret_hash,
            sync_enabled = 1,
            connection_state = ?,
            state_reason = '',
            last_connected_at = NULL,
            last_event_at = NULL,
            last_disconnect_at = NULL,
            last_user_activity_at = NULL,
            last_video_id = NULL,
            last_error = ''`,
		cuid.New(),
		now,
		now,
		userID,
		screenID,
		screenName,
		loungeTokenEnc,
		secretHash,
		true,
		youTubeTVSyncStateDisconnected,
		"",
		youTubeTVSyncStateDisconnected,
	)
	return err
}

func (c *sqliteClient) SetYouTubeTVSyncAccountEnabled(ctx context.Context, userID string, enabled bool) error {
	result, err := c.db.ExecContext(
		ctx,
		`UPDATE youtube_tv_sync_accounts
         SET sync_enabled = ?, updated_at = ?
         WHERE user_id = ?`,
		enabled,
		time.Now().UTC(),
		userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) UpdateYouTubeTVSyncLoungeToken(ctx context.Context, userID, screenID, screenName string, loungeTokenEnc []byte, secretHash string) error {
	result, err := c.db.ExecContext(
		ctx,
		`UPDATE youtube_tv_sync_accounts
         SET updated_at = ?,
             screen_id = ?,
             screen_name = ?,
             lounge_token_enc = ?,
             enc_secret_hash = ?
         WHERE user_id = ?`,
		time.Now().UTC(),
		screenID,
		screenName,
		loungeTokenEnc,
		secretHash,
		userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) DeleteYouTubeTVSyncAccount(ctx context.Context, userID string) error {
	result, err := c.db.ExecContext(ctx, `DELETE FROM youtube_tv_sync_accounts WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) ListEnabledYouTubeTVSyncAccounts(ctx context.Context, limit int) ([]*YouTubeTVSyncAccount, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := c.db.QueryContext(
		ctx,
		`SELECT id, created_at, updated_at, user_id, screen_id, screen_name, lounge_token_enc, enc_secret_hash, sync_enabled, connection_state, state_reason, last_connected_at, last_event_at, last_disconnect_at, last_user_activity_at, last_video_id, last_error
         FROM youtube_tv_sync_accounts
         WHERE sync_enabled = 1
         ORDER BY updated_at ASC
         LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*YouTubeTVSyncAccount
	for rows.Next() {
		account, err := scanYouTubeTVSyncAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (c *sqliteClient) UpdateYouTubeTVSyncState(ctx context.Context, userID string, update YouTubeTVSyncStateUpdate) error {
	result, err := c.db.ExecContext(
		ctx,
		`UPDATE youtube_tv_sync_accounts
         SET updated_at = ?,
             connection_state = ?,
             state_reason = ?,
             last_error = ?,
             last_connected_at = COALESCE(?, last_connected_at),
             last_event_at = COALESCE(?, last_event_at),
             last_disconnect_at = COALESCE(?, last_disconnect_at),
             last_user_activity_at = COALESCE(?, last_user_activity_at),
             last_video_id = COALESCE(?, last_video_id)
         WHERE user_id = ?`,
		time.Now().UTC(),
		update.ConnectionState,
		update.StateReason,
		update.LastError,
		nullableTime(update.LastConnectedAt),
		nullableTime(update.LastEventAt),
		nullableTime(update.LastDisconnectAt),
		nullableTime(update.LastUserActivity),
		nullableString(update.LastVideoID),
		userID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (c *sqliteClient) GetUserLastSessionActivity(ctx context.Context, userID string) (null.Time, error) {
	var lastUsed time.Time
	err := c.db.QueryRowContext(
		ctx,
		`SELECT last_used
         FROM sessions
         WHERE user_id = ? AND deleted = 0 AND expires_at > ?
         ORDER BY last_used DESC
         LIMIT 1`,
		userID,
		time.Now().UTC(),
	).Scan(&lastUsed)
	if err != nil {
		return null.Time{}, err
	}
	return null.TimeFrom(lastUsed.UTC()), nil
}

type scanner interface {
	Scan(dest ...any) error
}

func timePtrFromNull(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func stringPtrFromNull(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func nullableTime(t null.Time) any {
	if t.Valid {
		return t.Time.UTC()
	}
	return nil
}

func nullableString(s null.String) any {
	if s.Valid {
		return s.String
	}
	return nil
}
