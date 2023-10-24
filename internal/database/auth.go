package database

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/prisma/db"
)

func (c *Client) NewAuthNonce(expiration time.Time, value string) (*db.AuthNonceModel, error) {
	nonce, err := c.p.AuthNonce.CreateOne(db.AuthNonce.ExpiresAt.Set(expiration), db.AuthNonce.Value.Set(value)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return nonce, nil
}

func (c *Client) FindNonce(value string) (*db.AuthNonceModel, error) {
	nonce, err := c.p.AuthNonce.FindFirst(db.AuthNonce.Value.Equals(value), db.AuthNonce.ExpiresAt.Gt(time.Now())).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return nonce, nil
}
