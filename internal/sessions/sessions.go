package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
)

var ErrNotFound = errors.New("session not found")

type SessionClient struct {
	db database.SessionsClient
}

type Session struct {
	db database.SessionsClient

	Meta   map[string]string
	data   *models.Session
	exists bool
}

func New(db database.SessionsClient) (*SessionClient, error) {
	return &SessionClient{db: db}, nil
}

func (c *SessionClient) New(ctx context.Context) (Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		metrics.IncUserEvent("session_new", "error")
		return Session{exists: false}, err
	}

	var record models.Session
	record.Deleted = false
	record.ID = id.String()
	record.ExpiresAt = time.Now().Add(time.Hour * 24 * 7)

	session, err := c.db.CreateSession(ctx, &record)
	if err != nil {
		metrics.IncUserEvent("session_new", "error")
		return Session{exists: false}, err
	}
	metrics.IncUserEvent("session_new", "success")
	return Session{db: c.db, data: session, exists: true, Meta: make(map[string]string)}, nil
}

func (c *SessionClient) Get(ctx context.Context, id string) (Session, error) {
	session, err := c.db.GetSession(ctx, id)
	if err != nil {
		metrics.IncUserEvent("session_get", "error")
		return Session{exists: false}, err
	}

	var meta map[string]string = make(map[string]string)
	if session.Meta != nil && len(session.Meta) > 0 {
		err := json.Unmarshal(session.Meta, &meta)
		if err != nil {
			// Invalid meta JSON, just use empty map instead of deleting session
			meta = make(map[string]string)
		}
	}

	metrics.IncUserEvent("session_get", "success")
	return Session{Meta: meta, db: c.db, data: session, exists: true}, nil
}

func (c *SessionClient) Delete(ctx context.Context, id string) error {
	err := c.db.DeleteSession(ctx, id)
	if err != nil {
		metrics.IncUserEvent("session_delete", "error")
		return err
	}
	metrics.IncUserEvent("session_delete", "success")
	return nil
}

func (c Session) ID() string {
	return c.data.ID
}

func (c Session) Valid() bool {
	if !c.exists || c.data.ID == "" || c.db == nil {
		return false
	}
	// Check if session is expired
	if c.data.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

func (c Session) UpdateUser(ctx context.Context, userID null.String, connectionID null.String) (Session, error) {
	if !c.Valid() {
		return Session{exists: false}, errors.New("session does not exist")
	}

	err := c.db.UpdateSessionUser(ctx, c.data.ID, userID, connectionID)
	if err != nil {
		metrics.IncUserEvent("session_update_user", "error")
		return Session{exists: false}, err
	}

	c.data.UserID = userID
	c.data.ConnectionID = connectionID

	metrics.IncUserEvent("session_update_user", "success")
	return c, nil
}

func (c Session) UpdateMeta(ctx context.Context, meta map[string]string) (Session, error) {
	if !c.Valid() {
		return Session{exists: false}, errors.New("session does not exist")
	}

	err := c.db.UpdateSessionMeta(ctx, c.data.ID, meta)
	if err != nil {
		metrics.IncUserEvent("session_update_meta", "error")
		return Session{exists: false}, err
	}
	metrics.IncUserEvent("session_update_meta", "success")
	return c, nil
}

func (s Session) Refresh(ctx *fiber.Ctx) error {
	if !s.Valid() {
		return errors.New("session does not exist")
	}

	updated, err := s.db.SetSessionExpiration(ctx.Context(), s.data.ID, time.Now().Add(time.Hour*24*7))
	if err != nil {
		metrics.IncUserEvent("session_refresh", "error")
		return err
	}
	s.data = updated

	cookie, err := s.Cookie()
	if err != nil {
		metrics.IncUserEvent("session_refresh", "error")
		return err
	}
	ctx.Cookie(cookie)

	metrics.IncUserEvent("session_refresh", "success")
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
	host := os.Getenv("COOKIE_DOMAIN")
	// Strip port from domain - cookies don't use ports
	domain := strings.Split(host, ":")[0]
	secure := !strings.Contains(host, "localhost")

	return &fiber.Cookie{
		Name:    "session_id",
		Value:   s.data.ID,
		Expires: s.data.ExpiresAt,

		Secure:   secure,
		HTTPOnly: true,
		Path:     "/",
		Domain:   domain,
		SameSite: "lax",
	}, nil
}
