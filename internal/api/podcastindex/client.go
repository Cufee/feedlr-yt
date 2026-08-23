// Package podcastindex is a minimal client for the PodcastIndex.org API.
// It is only used for podcast discovery; episode data always comes from the
// podcasts' own RSS feeds.
package podcastindex

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/pkg/errors"
)

const (
	baseURL   = "https://api.podcastindex.org/api/1.0"
	userAgent = "Feedlr/1.0 (+https://feedlr.app)"
)

// Podcast is a search or lookup result for a podcast feed.
type Podcast struct {
	FeedID       int64
	FeedURL      string
	Title        string
	Description  string
	ArtworkURL   string
	Author       string
	EpisodeCount int
	Dead         bool
}

type client struct {
	key    string
	secret string
	http   *http.Client
}

// NewClient returns a client, or nil when key/secret are not configured
// (the feature is optional and callers must handle a nil client).
func NewClient(key, secret string) (*client, error) {
	if key == "" || secret == "" {
		return nil, nil
	}
	return &client{
		key:    key,
		secret: secret,
		http:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// SearchPodcasts searches the PodcastIndex catalog by term.
func (c *client) SearchPodcasts(ctx context.Context, query string, limit int) ([]Podcast, error) {
	if limit < 1 {
		limit = 4
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("max", strconv.Itoa(limit))

	var res struct {
		Status string    `json:"status"`
		Feeds  []apiFeed `json:"feeds"`
	}
	err := c.get(ctx, "/search/byterm", params, &res)
	if err != nil {
		return nil, err
	}

	podcasts := make([]Podcast, 0, len(res.Feeds))
	for _, feed := range res.Feeds {
		if podcast, ok := feed.toPodcast(); ok {
			podcasts = append(podcasts, podcast)
		}
	}
	return podcasts, nil
}

// PodcastByFeedURL resolves a feed URL to catalog metadata.
func (c *client) PodcastByFeedURL(ctx context.Context, feedURL string) (*Podcast, error) {
	params := url.Values{}
	params.Set("url", feedURL)

	var res struct {
		Status string   `json:"status"`
		Feed   *apiFeed `json:"feed"`
	}
	err := c.get(ctx, "/podcasts/byfeedurl", params, &res)
	if err != nil {
		return nil, err
	}
	if res.Feed == nil {
		return nil, nil
	}

	podcast, ok := res.Feed.toPodcast()
	if !ok {
		return nil, nil
	}
	return &podcast, nil
}

func (c *client) get(ctx context.Context, path string, params url.Values, out any) error {
	endpoint := baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	for header, value := range c.authHeaders() {
		req.Header.Set(header, value)
	}

	resp, err := c.http.Do(req)
	metrics.ObservePodcastAPICall("podcastindex", strings.TrimPrefix(path, "/"), err)
	if err != nil {
		return errors.Wrap(err, "podcastindex request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("podcastindex %s failed with status %d", path, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return errors.Wrap(err, "podcastindex read failed")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errors.Wrap(err, "podcastindex decode failed")
	}
	return nil
}

func (c *client) authHeaders() map[string]string {
	date := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha1.Sum([]byte(c.key + c.secret + date))
	return map[string]string{
		"User-Agent":    userAgent,
		"X-Auth-Date":   date,
		"X-Auth-Key":    c.key,
		"Authorization": hex.EncodeToString(sum[:]),
	}
}

type apiFeed struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	OriginalURL  string `json:"originalUrl"`
	Description  string `json:"description"`
	Author       string `json:"author"`
	OwnerName    string `json:"ownerName"`
	Image        string `json:"image"`
	Artwork      string `json:"artwork"`
	EpisodeCount int    `json:"episodeCount"`
	Dead         int    `json:"dead"`
	Locked       int    `json:"locked"`
}

func (f apiFeed) toPodcast() (Podcast, bool) {
	feedURL := f.URL
	if feedURL == "" {
		feedURL = f.OriginalURL
	}
	if feedURL == "" {
		return Podcast{}, false
	}

	author := f.Author
	if author == "" {
		author = f.OwnerName
	}

	artwork := f.Artwork
	if artwork == "" {
		artwork = f.Image
	}

	return Podcast{
		FeedID:       f.ID,
		FeedURL:      feedURL,
		Title:        f.Title,
		Description:  f.Description,
		ArtworkURL:   artwork,
		Author:       author,
		EpisodeCount: f.EpisodeCount,
		Dead:         f.Dead != 0,
	}, true
}
