package sponsorblock

import (
	"log"

	"github.com/cufee/feedlr-yt/internal/utils"
)

type client struct {
	apiUrl string
}

var DefaultClient *client
var C *client

func init() {
	apiUrl := utils.MustGetEnv("SPONSORBLOCK_API_URL")
	if apiUrl == "" {
		log.Fatal("SPONSORBLOCK_API_URL is empty")
	}

	DefaultClient = &client{
		apiUrl: apiUrl,
	}
	C = DefaultClient
}
