package database

import (
	"context"
)

func (c *Client) NewUser() (string, error) {
	user, err := c.p.User.CreateOne().Exec(context.TODO())
	if err != nil {
		return "", err
	}
	return user.ID, nil
}
