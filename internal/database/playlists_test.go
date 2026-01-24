package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/matryer/is"
)

func TestPlaylistsCRUD(t *testing.T) {
	is := is.New(t)

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		t.Skip("DATABASE_PATH not set")
	}

	c, err := NewSQLiteClient(dbPath)
	is.NoErr(err)
	defer c.Close()

	ctx := context.Background()
	db := c.(*sqliteClient).db

	// Create a test user first
	user := &models.User{
		ID:       "test-user-playlists",
		Username: "test-user-playlists",
	}
	err = user.Insert(ctx, db, boil.Infer())
	is.NoErr(err)

	// Clean up at the end
	defer func() {
		user.Delete(ctx, db)
	}()

	// Create a test channel
	channel := &models.Channel{
		ID:          "test-channel-playlists",
		Title:       "Test Channel",
		Description: "Test Description",
	}
	err = channel.Insert(ctx, db, boil.Infer())
	is.NoErr(err)
	defer channel.Delete(ctx, db)

	// Create a test video
	video := &models.Video{
		ID:          "test-video-playlists",
		Title:       "Test Video",
		Description: "Test Description",
		Duration:    100,
		ChannelID:   channel.ID,
		Type:        "video",
		PublishedAt: time.Now(),
	}
	err = video.Insert(ctx, db, boil.Infer())
	is.NoErr(err)
	defer video.Delete(ctx, db)

	// Test: Create a playlist
	playlist := NewWatchLaterPlaylist(user.ID)
	err = c.CreatePlaylist(ctx, playlist)
	is.NoErr(err)
	defer func() {
		playlist.Delete(ctx, c.(*sqliteClient).db)
	}()

	// Test: Get playlist by slug
	retrieved, err := c.GetPlaylistBySlug(ctx, user.ID, "watch-later")
	is.NoErr(err)
	is.True(retrieved.ID == playlist.ID)
	is.True(retrieved.Name == "Watch Later")
	is.True(retrieved.System == true)
	is.True(retrieved.TTLDays.Valid && retrieved.TTLDays.Int64 == 30)

	// Test: Video is not in playlist initially
	inPlaylist, err := c.IsVideoInPlaylist(ctx, playlist.ID, video.ID)
	is.NoErr(err)
	is.True(!inPlaylist)

	// Test: Add video to playlist
	err = c.AddPlaylistItem(ctx, playlist.ID, video.ID)
	is.NoErr(err)

	// Test: Video is now in playlist
	inPlaylist, err = c.IsVideoInPlaylist(ctx, playlist.ID, video.ID)
	is.NoErr(err)
	is.True(inPlaylist)

	// Test: Get playlist items
	items, err := c.GetPlaylistItems(ctx, playlist.ID)
	is.NoErr(err)
	is.True(len(items) == 1)
	is.True(items[0].VideoID == video.ID)

	// Test: Get playlist items with video loaded
	items, err = c.GetPlaylistItems(ctx, playlist.ID, PlaylistItem.WithVideo())
	is.NoErr(err)
	is.True(len(items) == 1)
	is.True(items[0].R != nil)
	is.True(items[0].R.Video != nil)
	is.True(items[0].R.Video.Title == "Test Video")

	// Test: Remove video from playlist
	err = c.RemovePlaylistItem(ctx, playlist.ID, video.ID)
	is.NoErr(err)

	// Test: Video is no longer in playlist
	inPlaylist, err = c.IsVideoInPlaylist(ctx, playlist.ID, video.ID)
	is.NoErr(err)
	is.True(!inPlaylist)

	t.Log("All playlist tests passed!")
}
