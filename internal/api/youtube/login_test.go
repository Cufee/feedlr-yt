package youtube

import (
	"context"
	"testing"

	"github.com/matryer/is"
)

func TestLoginFlow(t *testing.T) {
	is := is.New(t)

	client := NewOAuthClient()
	url, code, err := client.Authenticate(context.Background())
	is.NoErr(err)
	println("auth url:", url)
	println("auth code:", code)

	status := client.AuthStatus()
	is.True(status == AuthStatusAuthenticated)

	err = client.RefreshToken(context.Background())
	is.NoErr(err)
}
