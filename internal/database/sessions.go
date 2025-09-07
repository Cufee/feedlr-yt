package database

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/volatiletech/null/v8"
)

type SessionsClient interface {
	CreateSession(ctx context.Context, data *models.Session) (*models.Session, error)
	GetSession(ctx context.Context, id string) (*models.Session, error)
	UpdateSessionUser(ctx context.Context, id string, userID null.String, connectionID null.String) error
	UpdateSessionMeta(ctx context.Context, id string, meta map[string]string) error
	SetSessionExpiration(ctx context.Context, id string, expiresAt time.Time) (*models.Session, error)
	DeleteSession(ctx context.Context, id string) error
}

func (c *sqliteClient) GetSession(ctx context.Context, id string) (*models.Session, error) {
	session, err := models.Sessions(models.SessionWhere.ID.EQ(id), models.SessionWhere.ExpiresAt.GT(time.Now()), models.SessionWhere.Deleted.EQ(false)).One(ctx, c.db)
	if err != nil {
		return nil, err
	}

	session.LastUsed = time.Now()
	_, err = session.Update(ctx, c.db, boil.Whitelist(models.SessionColumns.LastUsed))
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (c *sqliteClient) CreateSession(ctx context.Context, session *models.Session) (*models.Session, error) {
	if session.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("invalid expiration time")
	}
	session.LastUsed = time.Now()

	err := session.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (c *sqliteClient) UpdateSessionUser(ctx context.Context, id string, userID null.String, connectionID null.String) error {
	session, err := c.GetSession(ctx, id)
	if err != nil {
		return err
	}

	session.UserID = userID
	session.ConnectionID = connectionID
	_, err = session.Update(ctx, c.db, boil.Whitelist(models.SessionColumns.UserID, models.SessionColumns.ConnectionID))
	if err != nil {
		return err
	}

	return nil
}

func (c *sqliteClient) UpdateSessionMeta(ctx context.Context, id string, meta map[string]string) error {
	session, err := c.GetSession(ctx, id)
	if err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	session.Meta = data
	_, err = session.Update(ctx, c.db, boil.Whitelist(models.SessionColumns.Meta))
	if err != nil {
		return err
	}

	return nil
}

func (c *sqliteClient) SetSessionExpiration(ctx context.Context, id string, expiresAt time.Time) (*models.Session, error) {
	session, err := c.GetSession(ctx, id)
	if err != nil {
		return nil, err
	}

	session.ExpiresAt = expiresAt
	_, err = session.Update(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (c *sqliteClient) DeleteSession(ctx context.Context, id string) error {
	session, err := c.GetSession(ctx, id)
	if err != nil {
		return err
	}

	session.Deleted = true
	_, err = session.Update(ctx, c.db, boil.Whitelist(models.SessionColumns.Deleted))
	if err != nil {
		return err
	}

	return nil
}
