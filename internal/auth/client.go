package auth

import (
	"context"
	"log"
	"os"

	"github.com/auth0/go-auth0/authentication"
)

var (
	domain       = os.Getenv("AUTH0_DOMAIN")
	clientID     = os.Getenv("AUTH0_CLIENT_ID")
	clientSecret = os.Getenv("AUTH0_CLIENT_SECRET")

	client *authentication.Authentication
)

func init() {
	// Initialize a new client using a domain, client ID and client secret.
	authAPI, err := authentication.New(
		context.Background(),
		domain,
		authentication.WithClientID(clientID),
		authentication.WithClientSecret(clientSecret),
	)
	if err != nil {
		log.Fatalf("failed to initialize the auth0 authentication API client: %+v", err)
	}
	client = authAPI
}
