package sessions

import (
	"context"
	"errors"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
	"github.com/volatiletech/null/v8"
)

var ErrNotFound = errors.New("session not found")

type SessionClient struct {
	db database.SessionsClient
}

type Session struct {
	db database.SessionsClient

	data   *models.Session
	exists bool
}

func New(db database.SessionsClient) (*SessionClient, error) {
	return &SessionClient{db: db}, nil
}

func (c *SessionClient) New(ctx context.Context) (Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return Session{exists: false}, err
	}

	var record models.Session
	record.ID = id.String()
	record.ExpiresAt = time.Now().Add(time.Hour * 24 * 7)

	session, err := c.db.CreateSession(ctx, &record)
	if err != nil {
		return Session{exists: false}, err
	}
	return Session{db: c.db, data: session, exists: true}, nil
}

func (c *SessionClient) Get(ctx context.Context, id string) (Session, error) {
	session, err := c.db.GetSession(ctx, id)
	if err != nil {
		return Session{exists: false}, err
	}
	return Session{db: c.db, data: session, exists: true}, nil
}

func (c *SessionClient) Delete(ctx context.Context, id string) error {
	return c.db.DeleteSession(ctx, id)
}

func (c Session) Valid() bool {
	return c.exists && c.data.ID != "" && c.db != nil
}

func (c Session) UpdateUser(ctx context.Context, userID null.String, connectionID null.String) (Session, error) {
	if !c.Valid() {
		return Session{exists: false}, errors.New("session does not exist")
	}

	err := c.db.UpdateSessionUser(ctx, c.data.ID, userID, connectionID)
	if err != nil {
		return Session{exists: false}, err
	}
	return c, nil
}

func (s Session) Refresh(ctx context.Context) error {
	if !s.Valid() {
		return errors.New("session does not exist")
	}

	_, err := s.db.SetSessionExpiration(ctx, s.data.ID, time.Now().Add(time.Hour*24*7))
	if err != nil {
		return err
	}
	return nil
}

/* Returns a session user ID and a bool indicating if a session is authenticated */
func (s Session) UserID() (string, bool) {
	if !s.Valid() {
		return "", false
	}

	if s.data.UserID.Valid {
		return s.data.UserID.String, true
	}
	return "", false
}

func (s Session) Cookie() (*fiber.Cookie, error) {
	return &fiber.Cookie{
		Name:     "session_id",
		Value:    s.data.ID,
		Expires:  s.data.ExpiresAt,
		HTTPOnly: true,
		SameSite: "lax",
	}, nil
}
