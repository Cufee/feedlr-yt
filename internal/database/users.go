package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

type UsersClient interface {
	CreateUser(ctx context.Context) (*models.User, error)
	GetOrCreateUser(ctx context.Context, id string) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error

	GetUserPasskeys(ctx context.Context, userID string) ([]*models.Passkey, error)
	SaveUserPasskey(ctx context.Context, key *models.Passkey) error
}

func (c *sqliteClient) GetOrCreateUser(ctx context.Context, id string) (*models.User, error) {
	user, err := c.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return c.CreateUser(ctx)
	}
	return user, err
}

func (c *sqliteClient) GetUser(ctx context.Context, id string) (*models.User, error) {
	user, err := models.FindUser(ctx, c.db, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (c *sqliteClient) CreateUser(ctx context.Context) (*models.User, error) {
	user := models.User{}
	err := user.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *sqliteClient) UpdateUser(ctx context.Context, user *models.User) error {
	_, err := user.Update(ctx, c.db, boil.Blacklist(models.UserColumns.ID))
	return err
}

func (c *sqliteClient) GetUserPasskeys(ctx context.Context, userID string) ([]*models.Passkey, error) {
	passkeys, err := models.Passkeys(models.PasskeyWhere.UserID.EQ(userID)).All(ctx, c.db)
	if err != nil {
		return nil, err
	}
	return passkeys, nil
}

func (c *sqliteClient) SaveUserPasskey(ctx context.Context, key *models.Passkey) error {
	err := key.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return err
	}
	return nil
}
