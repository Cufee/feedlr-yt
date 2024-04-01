package sponsorblock

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/cufee/feedlr-yt/internal/utils"
)

type client struct {
	apiUrl string
	http   http.Client
}

var DefaultClient *client
var C *client

var ErrRequestTimeout = errors.New("request timeout")

func init() {
	apiUrl := utils.MustGetEnv("SPONSORBLOCK_API_URL")
	if apiUrl == "" {
		log.Fatal("SPONSORBLOCK_API_URL is empty")
	}

	DefaultClient = &client{
		http:   http.Client{Timeout: time.Millisecond * 500},
		apiUrl: apiUrl,
	}
	C = DefaultClient
}
