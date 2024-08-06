package database

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

type AuthNonceClient interface {
	NewAuthNonce(ctx context.Context, expiration time.Time, value string) (*models.AuthNonce, error)
	FindNonce(ctx context.Context, value string) (*models.AuthNonce, error)
}

func (c *sqliteClient) NewAuthNonce(ctx context.Context, expiration time.Time, value string) (*models.AuthNonce, error) {
	nonce := models.AuthNonce{
		ID:        ensureID(""),
		Used:      false,
		Value:     value,
		ExpiresAt: expiration,
	}

	err := nonce.Insert(ctx, c.db, boil.Infer())
	if err != nil {
		return nil, err
	}
	return &nonce, nil
}

func (c *sqliteClient) FindNonce(ctx context.Context, value string) (*models.AuthNonce, error) {
	nonce, err := models.AuthNonces(models.AuthNonceWhere.Value.EQ(value), models.AuthNonceWhere.Used.EQ(false)).One(ctx, c.db)
	if err != nil {
		return nil, err
	}

	nonce.Used = true
	_, err = nonce.Update(ctx, c.db, boil.Whitelist(models.AuthNonceColumns.Used))
	if err != nil {
		return nil, err
	}

	nonce.Used = false // set used to false in case of some check down the line
	return nonce, nil
}
