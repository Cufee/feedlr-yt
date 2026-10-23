package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/watchstate"
)

type stateConn struct{ *sql.Conn }

func (c *stateConn) Exec(q string, args ...any) (sql.Result, error) {
	return c.ExecContext(context.Background(), q, args...)
}
func (c *stateConn) Query(q string, args ...any) (*sql.Rows, error) {
	return c.QueryContext(context.Background(), q, args...)
}
func (c *stateConn) QueryRow(q string, args ...any) *sql.Row {
	return c.QueryRowContext(context.Background(), q, args...)
}

type FrontendStateClient interface {
	SetWatchLaterMembership(context.Context, string, string, bool) ([]string, error)
	SetPlaylistMembership(context.Context, string, string, string, bool) ([]string, error)
	PatchViewState(context.Context, string, string, ViewStatePatch) (ViewState, error)
	PatchUserSettings(context.Context, string, SettingsPatch) error
}

type ViewStatePatch struct {
	Progress *int
	Hidden   *bool
}

type ViewState struct {
	Progress int
	Hidden   bool
}

type SettingsPatch struct {
	SponsorEnabled          *bool
	SponsorCategories       *[]string
	PodcastEnabled          *bool
	PodcastCategories       *[]string
	Volume                  *int
	FeedMode                *string
	ToggleSponsorEnabled    bool
	TogglePodcastEnabled    bool
	ToggleSponsorCategory   string
	TogglePodcastCategory   string
	SponsorCategoryDefaults []string
	PodcastCategoryDefaults []string
}

// GetOrCreateUserSettings resolves simultaneous first visits without replacing
// a record another request has already initialized or edited.
func (c *sqliteClient) GetOrCreateUserSettings(ctx context.Context, userID string, defaults []byte) (record *models.Setting, err error) {
	err = c.immediate(ctx, func(conn *stateConn) error {
		var findErr error
		record, findErr = models.Settings(models.SettingWhere.UserID.EQ(userID)).One(ctx, conn)
		if findErr == nil {
			return nil
		}
		if !errors.Is(findErr, sql.ErrNoRows) {
			return findErr
		}
		record = &models.Setting{UserID: userID, Data: defaults}
		return record.Insert(ctx, conn, boil.Infer())
	})
	return
}

// PatchUserSettings updates only submitted preferences under the database lock.
// In particular, a concurrent volume save cannot replace a category edit.
func (c *sqliteClient) PatchUserSettings(ctx context.Context, userID string, patch SettingsPatch) error {
	return c.immediate(ctx, func(conn *stateConn) error {
		record, err := models.Settings(models.SettingWhere.UserID.EQ(userID)).One(ctx, conn)
		if err != nil {
			return err
		}
		var settings map[string]json.RawMessage
		if err = json.Unmarshal(record.Data, &settings); err != nil {
			return err
		}
		if settings == nil {
			settings = make(map[string]json.RawMessage)
		}
		set := func(group, field string, value any) error {
			var fields map[string]json.RawMessage
			if raw := settings[group]; len(raw) > 0 {
				if err := json.Unmarshal(raw, &fields); err != nil {
					return err
				}
			}
			if fields == nil {
				fields = make(map[string]json.RawMessage)
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return err
			}
			fields[field] = encoded
			settings[group], err = json.Marshal(fields)
			return err
		}
		read := func(group, field string, value any) (bool, error) {
			var fields map[string]json.RawMessage
			if raw := settings[group]; len(raw) > 0 {
				if err := json.Unmarshal(raw, &fields); err != nil {
					return false, err
				}
			}
			raw := fields[field]
			if len(raw) == 0 || string(raw) == "null" {
				return false, nil
			}
			return true, json.Unmarshal(raw, value)
		}
		for _, toggle := range []struct {
			enabled      bool
			group, field string
		}{{patch.ToggleSponsorEnabled, "SponsorBlock", "SponsorBlockEnabled"}, {patch.TogglePodcastEnabled, "PodcastSegments", "Enabled"}} {
			if !toggle.enabled {
				continue
			}
			current := true
			if _, err := read(toggle.group, toggle.field, &current); err != nil {
				return err
			}
			if err := set(toggle.group, toggle.field, !current); err != nil {
				return err
			}
		}
		for _, toggle := range []struct {
			category, group, field string
			defaults               []string
		}{{patch.ToggleSponsorCategory, "SponsorBlock", "SelectedSponsorBlockCategories", patch.SponsorCategoryDefaults}, {patch.TogglePodcastCategory, "PodcastSegments", "SelectedCategories", patch.PodcastCategoryDefaults}} {
			if toggle.category == "" {
				continue
			}
			current := append([]string{}, toggle.defaults...)
			if _, err := read(toggle.group, toggle.field, &current); err != nil {
				return err
			}
			changed := []string{}
			found := false
			for _, category := range current {
				if category == toggle.category {
					found = true
				} else {
					changed = append(changed, category)
				}
			}
			if !found {
				changed = append(changed, toggle.category)
			}
			if err := set(toggle.group, toggle.field, changed); err != nil {
				return err
			}
		}
		if patch.SponsorEnabled != nil {
			if err = set("SponsorBlock", "SponsorBlockEnabled", *patch.SponsorEnabled); err != nil {
				return err
			}
		}
		if patch.SponsorCategories != nil {
			if err = set("SponsorBlock", "SelectedSponsorBlockCategories", *patch.SponsorCategories); err != nil {
				return err
			}
		}
		if patch.PodcastEnabled != nil {
			if err = set("PodcastSegments", "Enabled", *patch.PodcastEnabled); err != nil {
				return err
			}
		}
		if patch.PodcastCategories != nil {
			if err = set("PodcastSegments", "SelectedCategories", *patch.PodcastCategories); err != nil {
				return err
			}
		}
		if patch.Volume != nil {
			settings["PlayerVolume"], err = json.Marshal(*patch.Volume)
			if err != nil {
				return err
			}
		}
		if patch.FeedMode != nil {
			settings["FeedMode"], err = json.Marshal(*patch.FeedMode)
			if err != nil {
				return err
			}
		}
		record.Data, err = json.Marshal(settings)
		if err != nil {
			return err
		}
		_, err = record.Update(ctx, conn, boil.Whitelist(models.SettingColumns.Data))
		return err
	})
}

// immediate serializes read/modify/write transactions across SQLite connections
// and processes. A deferred transaction could fail when upgraded after a read.
func (c *sqliteClient) immediate(ctx context.Context, change func(*stateConn) error) error {
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
	}()
	if err = change(&stateConn{conn}); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}

func (c *sqliteClient) SetWatchLaterMembership(ctx context.Context, userID, mediaID string, present bool) (affected []string, err error) {
	err = c.immediate(ctx, func(conn *stateConn) error {
		p, findErr := models.Playlists(models.PlaylistWhere.UserID.EQ(userID), models.PlaylistWhere.Slug.EQ("watch-later")).One(ctx, conn)
		if errors.Is(findErr, sql.ErrNoRows) {
			if !present {
				affected = []string{mediaID}
				return nil
			}
			p = NewWatchLaterPlaylist(userID)
			if findErr = p.Insert(ctx, conn, boil.Infer()); findErr != nil {
				return findErr
			}
		} else if findErr != nil {
			return findErr
		}
		affected, findErr = setMembership(ctx, conn, p, mediaID, present)
		return findErr
	})
	return
}

func (c *sqliteClient) SetPlaylistMembership(ctx context.Context, userID, playlistID, mediaID string, present bool) (affected []string, err error) {
	err = c.immediate(ctx, func(conn *stateConn) error {
		p, findErr := models.FindPlaylist(ctx, conn, playlistID)
		if findErr != nil {
			return findErr
		}
		if p.UserID != userID || p.System {
			return sql.ErrNoRows
		}
		affected, findErr = setMembership(ctx, conn, p, mediaID, present)
		return findErr
	})
	return
}

func setMembership(ctx context.Context, conn *stateConn, p *models.Playlist, mediaID string, present bool) ([]string, error) {
	affected := []string{mediaID}
	var exists bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM playlist_items WHERE playlist_id=? AND video_id=?)", p.ID, mediaID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == present {
		return affected, nil
	}
	if !present {
		if _, err := conn.ExecContext(ctx, "DELETE FROM playlist_items WHERE playlist_id=? AND video_id=?", p.ID, mediaID); err != nil {
			return nil, err
		}
	} else {
		var available bool
		if err := conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM videos WHERE id=?)", mediaID).Scan(&available); err != nil {
			return nil, err
		}
		if !available {
			return nil, sql.ErrNoRows
		}
		if p.MaxSize.Valid && p.MaxSize.Int64 > 0 {
			var count int64
			if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM playlist_items WHERE playlist_id=?", p.ID).Scan(&count); err != nil {
				return nil, err
			}
			if count >= p.MaxSize.Int64 {
				rows, err := conn.QueryContext(ctx, "SELECT video_id FROM playlist_items WHERE playlist_id=? ORDER BY created_at ASC,id ASC LIMIT ?", p.ID, count-p.MaxSize.Int64+1)
				if err != nil {
					return nil, err
				}
				for rows.Next() {
					var id string
					if err = rows.Scan(&id); err != nil {
						rows.Close()
						return nil, err
					}
					affected = append(affected, id)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return nil, err
				}
				for _, id := range affected[1:] {
					if _, err = conn.ExecContext(ctx, "DELETE FROM playlist_items WHERE playlist_id=? AND video_id=?", p.ID, id); err != nil {
						return nil, err
					}
				}
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO playlist_items(id,playlist_id,video_id,position,created_at,updated_at) SELECT ?,?,?,COALESCE(MAX(position),-1)+1,datetime('now'),datetime('now') FROM playlist_items WHERE playlist_id=?", ensureID(""), p.ID, mediaID, p.ID); err != nil {
			return nil, err
		}
	}
	_, err := conn.ExecContext(ctx, "UPDATE playlists SET updated_at=datetime('now') WHERE id=?", p.ID)
	return affected, err
}

func (c *sqliteClient) PatchViewState(ctx context.Context, userID, mediaID string, patch ViewStatePatch) (state ViewState, err error) {
	err = c.immediate(ctx, func(conn *stateConn) error {
		var duration int
		if err := conn.QueryRowContext(ctx, "SELECT duration FROM videos WHERE id=?", mediaID).Scan(&duration); err != nil {
			return err
		}
		err := conn.QueryRowContext(ctx, "SELECT progress,COALESCE(hidden,0) FROM views WHERE user_id=? AND video_id=?", userID, mediaID).Scan(&state.Progress, &state.Hidden)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if patch.Progress != nil {
			state.Progress = max(0, *patch.Progress)
		}
		if patch.Hidden != nil {
			state.Hidden = *patch.Hidden
		}
		_, err = conn.ExecContext(ctx, "INSERT INTO views(id,user_id,video_id,progress,hidden,created_at,updated_at) VALUES(?,?,?,?,?,datetime('now'),datetime('now')) ON CONFLICT(user_id,video_id) DO UPDATE SET progress=excluded.progress,hidden=excluded.hidden,updated_at=excluded.updated_at", ensureID(""), userID, mediaID, state.Progress, state.Hidden)
		if err != nil {
			return err
		}
		if patch.Progress != nil && watchstate.IsCompleted(state.Progress, duration) {
			_, err = conn.ExecContext(ctx, "DELETE FROM playlist_items WHERE video_id=? AND playlist_id IN (SELECT id FROM playlists WHERE user_id=? AND slug='watch-later')", mediaID, userID)
		}
		return err
	})
	return
}
