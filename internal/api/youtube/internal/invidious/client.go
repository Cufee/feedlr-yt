package invidious

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	_ "github.com/joho/godotenv/autoload"
)

type client struct {
	host       string
	httpClient *http.Client
}

func (c *client) buildVideoEmbedURL(videoID string) string {
	return fmt.Sprintf("https://www.youtube.com/embed/%v", videoID)
}

func (c *client) buildChannelURL(videoID string) string {
	return fmt.Sprintf("https://www.youtube.com/channel/%v", videoID)
}

func NewClient(host string) *client {
	if host == "" {
		log.Fatal("INVIDIOUS_HOST is not set")
	}

	return &client{
		host:       host,
		httpClient: &http.Client{},
	}
}

/*
Sends a request to the Invidious API and decodes the response into the target.
  - Response is always expected to be a JSON object
  - Method is always GET, Invidious API does not support other methods atm
*/
func (c *client) request(endpoint string, target interface{}, parameters map[string]string) error {
	path, err := url.JoinPath(c.host, endpoint)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("https://%s", path), nil)
	if err != nil {
		return err
	}

	query := request.URL.Query()
	for key, value := range parameters {
		query.Add(key, value)
	}
	request.URL.RawQuery = query.Encode()

	res, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	return json.NewDecoder(res.Body).Decode(target)
}
