// Package rss fetches and parses podcast RSS feeds.
package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/pkg/errors"
)

const (
	// maxFeedSize caps how many bytes of a feed we are willing to read.
	maxFeedSize = 10 << 20 // 10 MB
	userAgent   = "Feedlr/1.0 (+https://feedlr.app)"
)

// Show is the podcast-level metadata extracted from a feed.
type Show struct {
	Title       string
	Description string
	ImageURL    string
	Author      string
}

// Episode is a single feed item with the fields we persist.
type Episode struct {
	GUID        string
	Title       string
	Description string
	Duration    int // seconds
	PublishedAt time.Time
	MediaURL    string
	Transcript  *Transcript
}

// Transcript is the selected timed Podcasting 2.0 transcript source.
type Transcript struct {
	URL      string
	MIMEType string
	Language string
	Rel      string
}

// FetchResult is the outcome of a feed fetch.
type FetchResult struct {
	Show         Show
	Episodes     []Episode
	CanonicalURL string
	ETag         string
	LastModified string
	NotModified  bool
}

// ShowID derives a stable, URL-safe channel ID from a feed URL.
// Two subscriptions to the same feed always converge on the same channel.
func ShowID(feedURL string) string {
	sum := sha256.Sum256([]byte(feedURL))
	return "pc_" + hex.EncodeToString(sum[:])[:20]
}

// EpisodeID derives a stable, URL-safe episode ID from the feed URL and item GUID.
func EpisodeID(feedURL, guid string) string {
	sum := sha256.Sum256([]byte(feedURL + "\x00" + guid))
	return "pe_" + hex.EncodeToString(sum[:])[:20]
}

// ParseDuration parses itunes:duration values: plain seconds ("5035"),
// "MM:SS", "HH:MM:SS" or "DD:HH:MM:SS". Returns 0 when unparseable.
func ParseDuration(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	if !strings.Contains(raw, ":") {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 0 {
			return 0
		}
		return seconds
	}

	parts := strings.Split(raw, ":")
	if len(parts) > 4 {
		return 0
	}

	values := make([]int, len(parts))
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 {
			return 0
		}
		values[i] = value
	}

	switch len(values) {
	case 2: // MM:SS
		return values[0]*60 + values[1]
	case 3: // HH:MM:SS
		return values[0]*3600 + values[1]*60 + values[2]
	case 4: // DD:HH:MM:SS
		return values[0]*86400 + values[1]*3600 + values[2]*60 + values[3]
	default:
		return 0
	}
}

// FetchFeed fetches feedURL with conditional GET headers (etag, lastModified)
// and parses the response. When the server answers 304 Not Modified the
// result only carries NotModified and the caller should keep its state.
func FetchFeed(ctx context.Context, feedURL, etag, lastModified string) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "invalid feed url")
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "feed request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{
			CanonicalURL: canonicalURL(resp, feedURL),
			NotModified:  true,
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("feed request failed with status %d", resp.StatusCode)
	}

	result, err := parseFeed(io.LimitReader(resp.Body, maxFeedSize))
	if err != nil {
		return nil, err
	}
	result.CanonicalURL = canonicalURL(resp, feedURL)
	result.ETag = resp.Header.Get("ETag")
	result.LastModified = resp.Header.Get("Last-Modified")
	return result, nil
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func canonicalURL(resp *http.Response, fallback string) string {
	if resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.String() != "" {
		return resp.Request.URL.String()
	}
	return fallback
}

func parseFeed(reader io.Reader) (*FetchResult, error) {
	feed, err := gofeed.NewParser().Parse(reader)
	if err != nil {
		return nil, errors.Wrap(err, "feed parse failed")
	}

	result := &FetchResult{Show: Show{Title: feed.Title}}

	if feed.Description != "" {
		result.Show.Description = feed.Description
	} else if feed.ITunesExt != nil && feed.ITunesExt.Summary != "" {
		result.Show.Description = feed.ITunesExt.Summary
	}
	if feed.Image != nil && feed.Image.URL != "" {
		result.Show.ImageURL = feed.Image.URL
	} else if feed.ITunesExt != nil && feed.ITunesExt.Image != "" {
		result.Show.ImageURL = feed.ITunesExt.Image
	}
	if feed.ITunesExt != nil {
		result.Show.Author = feed.ITunesExt.Author
	}
	if result.Show.Author == "" && feed.Author != nil {
		result.Show.Author = feed.Author.Name
	}
	if result.Show.Title == "" {
		result.Show.Title = result.Show.Author
	}

	for _, item := range feed.Items {
		episode, ok := episodeFromItem(item)
		if !ok {
			continue
		}
		result.Episodes = append(result.Episodes, episode)
	}

	// Feeds are expected newest-first, but do not rely on it.
	sortEpisodesByPublishedDesc(result.Episodes)
	return result, nil
}

func episodeFromItem(item *gofeed.Item) (Episode, bool) {
	var episode Episode

	if item.GUID == "" && item.Link != "" {
		episode.GUID = item.Link
	} else {
		episode.GUID = item.GUID
	}
	if episode.GUID == "" {
		return episode, false
	}

	enclosureURL := ""
	for _, enclosure := range item.Enclosures {
		if enclosure.URL == "" {
			continue
		}
		if !strings.HasPrefix(enclosure.Type, "audio/") && !strings.HasPrefix(enclosure.Type, "video/") && enclosure.Type != "" {
			continue
		}
		enclosureURL = enclosure.URL
		break
	}
	if enclosureURL == "" {
		// Some feeds only carry a plain link to the media file.
		if item.Link != "" && looksLikeMediaLink(item.Link) {
			enclosureURL = item.Link
		}
	}
	if enclosureURL == "" {
		return episode, false
	}

	publishedAt := item.PublishedParsed
	if publishedAt == nil {
		publishedAt = item.UpdatedParsed
	}
	if publishedAt == nil || publishedAt.IsZero() {
		return episode, false
	}

	episode.Title = strings.TrimSpace(item.Title)
	episode.Description = item.Description
	episode.PublishedAt = *publishedAt
	episode.MediaURL = enclosureURL
	if item.ITunesExt != nil {
		episode.Duration = ParseDuration(item.ITunesExt.Duration)
	}
	if episode.Duration == 0 && item.Extensions["yt"] != nil && len(item.Extensions["yt"]["duration"]) > 0 {
		// yt:duration is a common non-itunes extension carrying seconds.
		episode.Duration = ParseDuration(item.Extensions["yt"]["duration"][0].Value)
	}
	if extensions := item.Extensions["podcast"]; extensions != nil {
		for _, preferred := range []string{"text/vtt", "application/x-subrip", "application/srt"} {
			for _, extension := range extensions["transcript"] {
				mime := strings.ToLower(strings.TrimSpace(extension.Attrs["type"]))
				url := strings.TrimSpace(extension.Attrs["url"])
				if url == "" || mime != preferred {
					continue
				}
				episode.Transcript = &Transcript{URL: url, MIMEType: mime, Language: strings.TrimSpace(extension.Attrs["language"]), Rel: strings.TrimSpace(extension.Attrs["rel"])}
				break
			}
			if episode.Transcript != nil {
				break
			}
		}
	}

	return episode, true
}

func looksLikeMediaLink(link string) bool {
	lower := strings.ToLower(link)
	for _, ext := range []string{".mp3", ".m4a", ".mp4", ".ogg", ".opus", ".wav", ".aac", ".flac"} {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}

func sortEpisodesByPublishedDesc(episodes []Episode) {
	slices.SortFunc(episodes, func(a, b Episode) int {
		return b.PublishedAt.Compare(a.PublishedAt)
	})
}
