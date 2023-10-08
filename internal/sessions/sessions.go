package sessions

import (
	"errors"
	"time"

	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/prisma/db"
	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
)

var ErrNotFound = errors.New("session not found")

type Options = database.SessionOptions

type Session struct {
	ID   string
	data *db.SessionModel
}

func (s *Session) fetch() error {
	if s == nil {
		return ErrNotFound
	}
	if s.ID == "" {
		return errors.New("session ID is empty")
	}
	if s.data != nil {
		return nil
	}

	session, err := database.C.GetSession(s.ID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	s.data = session
	return nil
}

func New() (Session, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return Session{}, err
	}

	session, err := database.C.NewSession(database.SessionOptions{
		ID:        id.String(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
	})
	if err != nil {
		return Session{}, err
	}

	return Session{
		ID:   session.ID,
		data: session,
	}, nil
}

func FromID(id string) (Session, error) {
	session, err := database.C.GetSession(id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}

	return Session{
		ID:   session.ID,
		data: session,
	}, nil
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) Valid() bool {
	err := s.fetch()
	if err != nil {
		return false
	}
	if _, ok := s.data.UserID(); s.data.ExpiresAt.After(time.Now()) && ok {
		go s.Update(Options{LastUsed: time.Now()})
		return true
	}
	return false
}

/* Sets the session expiration time to 7 days from now */
func (s *Session) Refresh() error {
	return s.Update(Options{ExpiresAt: time.Now().Add(time.Hour * 24 * 7)})
}

/* Finds a valid session by ID and returns the user ID associated with it */
func (s *Session) UserID() (string, bool) {
	if ok := s.Valid(); ok {
		return s.data.UserID()
	}
	return "", false
}

func (s *Session) Cookie() (*fiber.Cookie, error) {
	err := s.fetch()
	if err != nil {
		return nil, err
	}
	return &fiber.Cookie{
		Name:     "session_id",
		Value:    s.ID,
		Expires:  s.data.ExpiresAt,
		HTTPOnly: true,
		SameSite: "Lax",
	}, nil
}

func (s *Session) Update(session Options) error {
	err := s.fetch()
	if err != nil {
		return err
	}

	s.data, err = database.C.UpdateSession(s.ID, session)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Session) Delete() error {
	err := s.fetch()
	if err != nil {
		return err
	}
	err = database.C.DeleteSession(s.ID)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func DeleteSession(id string) error {
	err := database.C.DeleteSession(id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
