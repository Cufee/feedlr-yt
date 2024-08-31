package piped

import (
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	apiURL *url.URL
	http   *http.Client
}

func NewClient(apiURL string) (*Client, error) {
	c := &http.Client{
		Timeout: time.Second * 5,
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		apiURL: u,
		http:   c,
	}, nil
}
