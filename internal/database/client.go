package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type Client interface {
	AuthNonceClient
	SessionsClient

	ChannelsClient
	VideosClient
	ViewsClient

	UsersClient
	SettingsClient
	ConnectionsClient
	SubscriptionsClient

	Close() error
}

func NewSQLiteClient(path string) (Client, error) {
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
