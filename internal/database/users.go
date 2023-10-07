package database

import (
	"context"
	"errors"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) EnsureUserExists(authId string) (*db.UserModel, error) {
	user, err := c.GetUserFromAuthID(authId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return c.NewUser(authId)
		}
		return nil, err
	}
	return user, nil
}

func (c *Client) GetUserFromAuthID(authId string) (*db.UserModel, error) {
	user, err := c.p.User.FindUnique(db.User.AuthID.Equals(authId)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (c *Client) NewUser(authId string) (*db.UserModel, error) {
	user, err := c.p.User.CreateOne(db.User.AuthID.Set(authId)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return user, nil
}
