package database

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/aarondl/sqlboiler/v4/boil"
	_ "github.com/mattn/go-sqlite3"
)

type Client interface {
	SessionsClient

	ChannelsClient
	VideosClient
	ViewsClient

	UsersClient
	SettingsClient
	SubscriptionsClient

	PlaylistsClient

	ConfigurationClient
	YouTubeSyncClient
	YouTubeTVSyncClient

	Close() error
}

func NewSQLiteClient(path string) (Client, error) {
	boil.DebugMode = os.Getenv("DEBUG_SQL") == "true"

	sqldb, err := sql.Open("sqlite3", fmt.Sprintf("file://%s?_fk=1&_auto_vacuum=2&_synchronous=1&_journal_mode=WAL", path)) // _mutex
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1)

	return &sqliteClient{
		db: sqldb,
	}, nil
}

type sqliteClient struct {
	db *sql.DB
}

func (c *sqliteClient) Close() error {
	return c.db.Close()
}

func toAny[T any](v []T) []any {
	var s []any
	for _, v := range v {
		s = append(s, v)
	}
	return s
}
