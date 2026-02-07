package sponsorblock

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	apiUrl string
	http   http.Client
}

var DefaultClient *client
var C *client

var ErrRequestTimeout = errors.New("request timeout")

func init() {
	apiUrl := strings.TrimSpace(os.Getenv("SPONSORBLOCK_API_URL"))
	if apiUrl == "" {
		log.Println("sponsorblock disabled: SPONSORBLOCK_API_URL is not set")
		return
	}

	DefaultClient = &client{
		http:   http.Client{Timeout: time.Millisecond * 500},
		apiUrl: apiUrl,
	}
	C = DefaultClient
}
