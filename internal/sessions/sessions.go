package sessions

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
)

var ErrNotFound = errors.New("session not found")

type Session struct {
	ID   string
	data SessionData
}
type SessionData struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	AuthId string `json:"auth_id"`

	ExpiresAt time.Time `json:"expires_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastUsed  time.Time `json:"last_used"`
}

func New() (*Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return &Session{}, err
	}

	var data SessionData
	data.ID = id.String()
	data.CreatedAt = time.Now()
	data.ExpiresAt = time.Now().Add(time.Hour * 24 * 7)

	err = defaultClient.Set("sessions", data.ID, data)
	if err != nil {
		return &Session{}, err
	}
	return &Session{data: data, ID: data.ID}, nil
}

func FromID(id string) (*Session, error) {
	var found []SessionData
	err := defaultClient.Get("sessions", id, &found)
	if err != nil {
		return &Session{}, err
	}
	if len(found) == 0 {
		return &Session{}, ErrNotFound
	}
	if len(found) > 1 {
		return &Session{}, errors.New("multiple sessions found")
	}

	return &Session{data: found[0]}, nil
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) Valid() bool {
	if s.data.UserID != "" && s.data.ExpiresAt.After(time.Now()) {
		s.data.LastUsed = time.Now()
		go s.save()
		return true
	}
	return false
}

/* Sets the session expiration time to 7 days from now */
func (s *Session) Refresh() error {
	s.data.ExpiresAt = time.Now().Add(time.Hour * 24 * 7)
	return s.save()
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) UserID() (string, bool) {
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

func (s *Session) AddUserID(userId, authId string) error {
	s.data.UserID = userId
	s.data.AuthId = authId
	return s.save()
}

func (s *Session) save() error {
	s.data.UpdatedAt = time.Now()
	return defaultClient.Set("sessions", s.data.ID, s.data)
}

func (s *Session) Delete() error {
	return defaultClient.Del("sessions", s.ID)
}

func DeleteSession(id string) error {
	return defaultClient.Del("sessions", id)
}
