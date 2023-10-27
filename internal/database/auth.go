package database

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/kamva/mgm/v3"
)

func (c *Client) NewAuthNonce(expiration time.Time, value string) (*models.AuthNonce, error) {
	nonce := models.NewAuthNonce(expiration, value)
	err := mgm.Coll(&models.AuthNonce{}).CreateWithCtx(mgm.Ctx(), nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}

func (c *Client) FindNonce(value string) (*models.AuthNonce, error) {
	nonce := &models.AuthNonce{}
	err := mgm.Coll(nonce).FindByID(value, nonce)
	if err != nil {
		return nil, err
	}
	return nonce, nil
}
