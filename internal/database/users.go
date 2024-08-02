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
