package auth

import (
	"context"
	"encoding/json"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/go-webauthn/webauthn/webauthn"
)

type user struct {
	db database.UsersClient
	*models.User
}

func (u *user) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u *user) WebAuthnName() string {
	if u.Username == "" {
		return u.ID
	}
	return u.Username
}

func (u *user) WebAuthnDisplayName() string {
	return u.WebAuthnName()
}

func (u *user) WebAuthnCredentials(ctx context.Context) []webauthn.Credential {
	passkeys, err := u.db.GetUserPasskeys(ctx, u.ID)
	if err != nil {
		return nil
	}

	var creds []webauthn.Credential
	for _, pk := range passkeys {
		var data webauthn.Credential
		err = json.Unmarshal(pk.Data, &data)
		if err != nil {
			continue
		}
		creds = append(creds, data)
	}
	return creds
}

type userStore struct {
	db database.UsersClient
}

func (s *userStore) GetOrCreateUser(ctx context.Context, userID string) (user, error) {
	record, err := s.db.GetOrCreateUser(ctx, userID)
	if err != nil {
		return user{db: s.db}, err
	}

	return user{db: s.db, User: record}, nil
}

func (s *userStore) SaveUser(ctx context.Context, user user) error {
	err := s.db.UpdateUser(ctx, user.User)
	if err != nil {
		return err
	}

	return nil
}
