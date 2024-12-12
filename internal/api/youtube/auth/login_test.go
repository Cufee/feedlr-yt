package auth

import (
	"context"
	"testing"

	"github.com/cufee/feedlr-yt/tests/mock"
	"github.com/matryer/is"
	"github.com/pkg/errors"
)

func testAuthClient() (*Client, error) {
	client := NewClient(&mock.AuthStore{})
	authed, err := client.Authenticate(context.Background(), true)
	if err != nil {
		return nil, err
	}
	<-authed

	status := client.AuthStatus()
	if status != AuthStatusAuthenticated {
		return nil, errors.New("bad auth status")
	}
	return client, nil
}

func TestLoginFlow(t *testing.T) {
	is := is.New(t)

	client, err := testAuthClient()
	is.NoErr(err)

	err = client.RefreshToken(context.Background())
	is.NoErr(err)
}
