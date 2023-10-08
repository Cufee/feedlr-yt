package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/byvko-dev/youtube-app/prisma/db"
	"github.com/steebchen/prisma-client-go/runtime/transaction"
)

type SessionOptions struct {
	ID        string
	UserID    string
	LastUsed  time.Time
	ExpiresAt time.Time
	Meta      map[string]interface{}

	AuthID         string
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt time.Time
}

/* NewSession creates a new session */
func (c *Client) NewSession(opts SessionOptions) (*db.SessionModel, error) {
	metaBytes, err := json.Marshal(opts.Meta)
	if err != nil {
		return nil, err
	}

	session, err := c.p.Session.CreateOne(db.Session.ID.Set(opts.ID), db.Session.ExpiresAt.Set(opts.ExpiresAt), db.Session.UserID.Set(opts.UserID), db.Session.AuthID.Set(opts.AuthID), db.Session.AccessToken.Set(opts.AccessToken), db.Session.RefreshToken.Set(opts.RefreshToken), db.Session.TokenExpiresAt.Set(opts.TokenExpiresAt), db.Session.Meta.Set(metaBytes)).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return session, nil
}

/* GetSession returns a session if it exists */
func (c *Client) GetSession(id string) (*db.SessionModel, error) {
	session, err := c.p.Session.FindUnique(db.Session.ID.Equals(id)).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return session, nil
}

/* GetValidSession returns a session if it exists and is not expired */
func (c *Client) GetValidSession(id string) (*db.SessionModel, error) {
	session, err := c.p.Session.FindFirst(db.Session.ID.Equals(id), db.Session.ExpiresAt.Gt(time.Now())).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return session, nil
}

/* GetSessionByAuthID returns a session if it exists */
func (c *Client) GetSessionByAuthID(authID string) (*db.SessionModel, error) {
	session, err := c.p.Session.FindFirst(db.Session.AuthID.Equals(authID)).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return session, nil
}

/* Updates a session using the provided options, all updated fields will be replaced */
func (c *Client) UpdateSession(id string, opts SessionOptions) (*db.SessionModel, error) {
	updateOpts, err := optionsToUpdate(opts)
	if err != nil {
		return nil, err
	}
	session, err := c.p.Session.FindUnique(db.Session.ID.Equals(id)).Update(updateOpts...).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (c *Client) DeleteSession(id string) error {
	_, err := c.p.Session.FindUnique(db.Session.ID.Equals(id)).Delete().Exec(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteExpiredSessions() error {
	_, err := c.p.Session.FindMany(db.Session.ExpiresAt.Lt(time.Now())).Delete().Exec(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteSessionsByAuthID(authID string) error {
	_, err := c.p.Session.FindMany(db.Session.AuthID.Equals(authID)).Delete().Exec(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) DeleteSessionsByUserID(userID string) error {
	_, err := c.p.Session.FindMany(db.Session.UserID.Equals(userID)).Delete().Exec(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) FindExpiringSessions(deadline time.Time) ([]db.SessionModel, error) {
	sessions, err := c.p.Session.FindMany(db.Session.ExpiresAt.Before(deadline)).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (c *Client) UpdateManySessions(updates map[string]SessionOptions) error {
	var queries []transaction.Param
	for id, opts := range updates {
		o, err := optionsToUpdate(opts)
		if err != nil {
			continue
		}
		tx := c.p.Session.FindUnique(db.Session.ID.Equals(id)).Update(o...)
		queries = append(queries, tx.Tx())
	}

	err := c.p.Prisma.Transaction(queries...).Exec(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func optionsToUpdate(opts SessionOptions) ([]db.SessionSetParam, error) {
	var updateOpts []db.SessionSetParam
	if opts.LastUsed != (time.Time{}) {
		updateOpts = append(updateOpts, db.Session.LastUsed.Set(opts.LastUsed))
	}
	if opts.ExpiresAt != (time.Time{}) {
		updateOpts = append(updateOpts, db.Session.ExpiresAt.Set(opts.ExpiresAt))
	}
	if opts.UserID != "" {
		updateOpts = append(updateOpts, db.Session.UserID.Set(opts.UserID))
	}
	if opts.AuthID != "" {
		updateOpts = append(updateOpts, db.Session.AuthID.Set(opts.AuthID))
	}
	if opts.AccessToken != "" {
		updateOpts = append(updateOpts, db.Session.AccessToken.Set(opts.AccessToken))
	}
	if opts.RefreshToken != "" {
		updateOpts = append(updateOpts, db.Session.RefreshToken.Set(opts.RefreshToken))
	}
	if opts.TokenExpiresAt != (time.Time{}) {
		updateOpts = append(updateOpts, db.Session.TokenExpiresAt.Set(opts.TokenExpiresAt))
	}
	if len(opts.Meta) > 0 {
		metaBytes, err := json.Marshal(opts.Meta)
		if err != nil {
			return nil, err
		}
		updateOpts = append(updateOpts, db.Session.Meta.Set(metaBytes))
	}
	return updateOpts, nil
}
