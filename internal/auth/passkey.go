package auth

import (
	"context"
	"encoding/json"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/friendsofgo/errors"
	"github.com/go-webauthn/webauthn/webauthn"
)

var _ webauthn.User = User{}

type userCredential struct {
	webauthn.Credential
	*models.Passkey
	updated bool
}

type User struct {
	*models.User
	credentials []userCredential
}

func (u User) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u User) WebAuthnName() string {
	if u.Username == "" {
		return u.ID
	}
	return u.Username
}

func (u User) WebAuthnDisplayName() string {
	return u.WebAuthnName()
}

func (u User) WebAuthnCredentials() []webauthn.Credential {
	var c []webauthn.Credential
	for _, uc := range u.credentials {
		c = append(c, uc.Credential)
	}
	return c
}

func (u *User) AddCredential(credential webauthn.Credential) error {
	for _, c := range u.credentials {
		if string(c.Credential.ID) == string(credential.ID) {
			return errors.New("duplicate credential")
		}
	}
	u.credentials = append(u.credentials, userCredential{Credential: credential, updated: true})
	return nil
}

func (u *User) UpdateCredential(credential webauthn.Credential) error {
	for _, c := range u.credentials {
		if string(c.Credential.ID) == string(credential.ID) {
			c.Credential = credential
			return nil
		}
	}
	return errors.New("credential not found")
}

type userStore struct {
	db database.UsersClient
}

func NewStore(db database.UsersClient) *userStore {
	return &userStore{db: db}
}

func (s *userStore) GetUser(ctx context.Context, userID string) (User, error) {
	record, err := s.db.GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}

	passkeys, err := s.db.GetUserPasskeys(ctx, record.ID)
	if err != nil && !database.IsErrNotFound(err) {
		return User{}, err
	}

	u := User{User: record}
	for _, pk := range passkeys {
		var data webauthn.Credential
		err = json.Unmarshal(pk.Data, &data)
		if err != nil {
			return User{}, errors.Wrap(err, "failed to decode credential")
		}
		u.credentials = append(u.credentials, userCredential{data, pk, false})
	}

	return u, nil
}

func (s *userStore) FindUser(ctx context.Context, username string) (User, error) {
	record, err := s.db.FindUser(ctx, username)
	if err != nil {
		return User{}, err
	}

	passkeys, err := s.db.GetUserPasskeys(ctx, record.ID)
	if err != nil && !database.IsErrNotFound(err) {
		return User{}, err
	}

	u := User{User: record}
	for _, pk := range passkeys {
		var data webauthn.Credential
		err = json.Unmarshal(pk.Data, &data)
		if err != nil {
			return User{}, errors.Wrap(err, "failed to decode credential")
		}
		u.credentials = append(u.credentials, userCredential{data, pk, false})
	}

	return u, nil
}

func (s *userStore) NewUser(ctx context.Context, userID string, username string) (User, error) {
	if userID == "" {
		return User{}, errors.New("userID cannot be left blank")
	}
	return User{User: &models.User{Username: username, ID: userID}}, nil
}

func (s *userStore) CreateUser(ctx context.Context, user *User) error {
	record, err := s.db.CreateUser(ctx, user.ID, user.Username)
	if err != nil {
		return err
	}
	user.User = record

	for _, c := range user.credentials {
		if !c.updated {
			continue
		}
		if c.Passkey == nil {
			c.Passkey = &models.Passkey{
				UserID: user.ID,
			}
		}

		data, err := json.Marshal(c.Credential)
		if err != nil {
			return errors.Wrap(err, "failed to encode credential")
		}
		c.Passkey.Data = data

		err = s.db.SaveUserPasskey(ctx, c.Passkey)
		if err != nil {
			return errors.Wrap(err, "failed to encode credential")
		}
	}

	return nil
}

func (s *userStore) SaveUser(ctx context.Context, user *User) error {
	err := s.db.UpdateUser(ctx, user.User)
	if err != nil {
		return err
	}

	for _, c := range user.credentials {
		if !c.updated {
			continue
		}
		if c.Passkey == nil {
			c.Passkey = &models.Passkey{
				UserID: user.ID,
			}
		}

		data, err := json.Marshal(c.Credential)
		if err != nil {
			return errors.Wrap(err, "failed to encode credential")
		}
		c.Passkey.Data = data

		err = s.db.SaveUserPasskey(ctx, c.Passkey)
		if err != nil {
			return errors.Wrap(err, "failed to encode credential")
		}

		c.updated = false
	}

	return nil
}
