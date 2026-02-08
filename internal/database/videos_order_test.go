package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestFindVideosOrdersByPublishedAtFirst(t *testing.T) {
	is := is.New(t)
	client := setupVideoOrderFixture(t)

	videos, err := client.FindVideos(context.Background(), Video.Channel("channel-1"), Video.Limit(4))
	is.NoErr(err)
	is.Equal(len(videos), 4)

	// Expected: published_at DESC, created_at DESC, id DESC.
	is.Equal(videos[0].ID, "video-c")
	is.Equal(videos[1].ID, "video-b")
	is.Equal(videos[2].ID, "video-a")
	is.Equal(videos[3].ID, "video-z-old-backfill")
}

func TestGetChannelWithVideosOrdersByPublishedAtFirst(t *testing.T) {
	is := is.New(t)
	client := setupVideoOrderFixture(t)

	channel, err := client.GetChannel(context.Background(), "channel-1", Channel.WithVideos(4))
	is.NoErr(err)
	is.Equal(len(channel.R.Videos), 4)

	// Expected: published_at DESC, created_at DESC, id DESC.
	is.Equal(channel.R.Videos[0].ID, "video-c")
	is.Equal(channel.R.Videos[1].ID, "video-b")
	is.Equal(channel.R.Videos[2].ID, "video-a")
	is.Equal(channel.R.Videos[3].ID, "video-z-old-backfill")
}

func setupVideoOrderFixture(t *testing.T) *sqliteClient {
	t.Helper()
	is := is.New(t)

	db, err := sql.Open("sqlite3", "file:video-order-fixture?mode=memory&_fk=1")
	is.NoErr(err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE channels (
			id TEXT PRIMARY KEY,
			created_at DATE NOT NULL,
			updated_at DATE NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			thumbnail TEXT NOT NULL,
			feed_updated_at DATE,
			uploads_playlist_id TEXT
		);
		CREATE TABLE videos (
			id TEXT PRIMARY KEY,
			created_at DATE NOT NULL,
			updated_at DATE NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL,
			duration INTEGER NOT NULL,
			published_at DATE NOT NULL,
			private BOOLEAN NOT NULL DEFAULT FALSE,
			type TEXT NOT NULL DEFAULT 'video',
			channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE
		);
	`)
	is.NoErr(err)

	now := time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC)
	_, err = db.Exec(
		`INSERT INTO channels (id, created_at, updated_at, title, description, thumbnail, feed_updated_at, uploads_playlist_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"channel-1",
		now,
		now,
		"Channel 1",
		"desc",
		"thumb.png",
		now,
		"upl-1",
	)
	is.NoErr(err)

	type videoRow struct {
		id          string
		createdAt   time.Time
		publishedAt time.Time
	}
	rows := []videoRow{
		{
			id:          "video-a",
			createdAt:   time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
			publishedAt: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
		},
		{
			id:          "video-z-old-backfill",
			createdAt:   time.Date(2026, 1, 16, 8, 0, 0, 0, time.UTC),
			publishedAt: time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC),
		},
		{
			id:          "video-b",
			createdAt:   time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
			publishedAt: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
		},
		{
			id:          "video-c",
			createdAt:   time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
			publishedAt: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC),
		},
	}

	for _, v := range rows {
		_, err = db.Exec(
			`INSERT INTO videos (id, created_at, updated_at, title, description, duration, published_at, private, type, channel_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			v.id,
			v.createdAt,
			now,
			"title",
			"desc",
			100,
			v.publishedAt,
			false,
			"video",
			"channel-1",
		)
		is.NoErr(err)
	}

	return &sqliteClient{db: db}
}
