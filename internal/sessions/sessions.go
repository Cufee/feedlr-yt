package sessions

import (
	"errors"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
)

var ErrNotFound = errors.New("session not found")

type SessionClient struct {
	db database.Client
}

type Session struct {
	ID     string
	data   SessionData
	exists bool
}
type SessionData struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	ConnectionID string `json:"connection_id"`

	ExpiresAt time.Time `json:"expires_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastUsed  time.Time `json:"last_used"`
}

func New() (*SessionClient, error) {
	return nil, nil
}

func (c *SessionClient) New() (*Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return &Session{}, errors.Join(errors.New("sessions.New"), err)
	}

	var data SessionData
	data.ID = id.String()
	data.CreatedAt = time.Now()
	data.ExpiresAt = time.Now().Add(time.Hour * 24 * 7)

	// err = c.db.Set("sessions", data.ID, data)
	// if err != nil {
	// return &Session{}, errors.Join(errors.New("sessions.New"), err)
	// }
	// return &Session{data: data, ID: data.ID}, nil
	return nil, nil
}

func (c *SessionClient) FromID(id string) (*Session, error) {
	var data SessionData
	// err := db.Get("sessions", id, &data)
	// if err != nil {
	// return &Session{}, err
	// }
	return &Session{data: data}, nil
}

func (c *SessionClient) Update(s *Session) error {
	s.data.UpdatedAt = time.Now()
	// return c.db.Set("sessions", s.data.ID, s.data)
	return nil
}

func (c *SessionClient) DeleteSession(id string) error {
	// return defaultClient.Del("sessions", id)
	return nil
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) Valid() bool {
	if !s.exists {
		return false
	}

	if s.data.UserID != "" && s.data.ExpiresAt.After(time.Now()) {
		return true
	}
	return false
}

func (s *Session) Refresh() {
	return
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) UserID() (string, bool) {
	if !s.exists {
		return "", false
	}

	if ok := s.Valid(); ok {
		return s.data.UserID, true
	}
	return "", false
}

func (s *Session) Cookie() (*fiber.Cookie, error) {
	return &fiber.Cookie{
		Name:     "session_id",
		Value:    s.ID,
		Expires:  s.data.ExpiresAt,
		HTTPOnly: true,
		SameSite: "strict",
	}, nil
}
