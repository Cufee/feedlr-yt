package sponsorblock

import (
	"log"
	"os"
)

type client struct {
	apiUrl string
}

var DefaultClient *client
var C *client

func init() {
	apiUrl := os.Getenv("SPONSORBLOCK_API_URL")
	if apiUrl == "" {
		log.Fatal("SPONSORBLOCK_API_URL is empty")
	}

	DefaultClient = &client{
		apiUrl: apiUrl,
	}
	C = DefaultClient
}
