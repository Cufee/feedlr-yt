package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/huandu/go-sqlbuilder"
)

var ErrUsernameNotAvailable = errors.New("username taken")

type UsersClient interface {
	CreateUser(ctx context.Context, userID string, username string) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	FindUser(ctx context.Context, username string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error

	GetUserPasskeys(ctx context.Context, userID string) ([]*models.Passkey, error)
	SaveUserPasskey(ctx context.Context, key *models.Passkey) error
	DeleteUserPasskey(ctx context.Context, userID string, id string) error
}

func (c *sqliteClient) GetUser(ctx context.Context, id string) (*models.User, error) {
	user, err := models.FindUser(ctx, c.db, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (c *sqliteClient) FindUser(ctx context.Context, username string) (*models.User, error) {
	users, err := models.Users(qm.Where(fmt.Sprintf("LOWER(%s) = ?", models.UserColumns.Username), strings.ToLower(username))).All(ctx, c.db)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, sql.ErrNoRows
	}
	if len(users) > 1 {
		return nil, errors.New("multiple users found")
	}
	return users[0], nil
}

func (c *sqliteClient) CreateUser(ctx context.Context, userID string, username string) (*models.User, error) {
	_, err := c.FindUser(ctx, username)
	if !IsErrNotFound(err) {
		return nil, ErrUsernameNotAvailable
	}

	user := models.User{Username: username, ID: userID}
	err = user.Insert(ctx, c.db, boil.Infer())
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

func (c *sqliteClient) DeleteUserPasskey(ctx context.Context, userID, id string) error {
	query := sqlbuilder.
		DeleteFrom(models.TableNames.Passkeys)
	query = query.Where(query.EQ(models.PasskeyColumns.ID, id), query.EQ(models.PasskeyColumns.UserID, userID))

	q, a := query.Build()
	deleted, err := models.Passkeys(qm.SQL(q, a...)).Exec(c.db)
	if err != nil {
		return err
	}
	if n, _ := deleted.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
