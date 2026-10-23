package database

import (
	"context"
	"net/url"
	"strings"
)

// Catalog URLs can redirect to a canonical RSS feed. Keep the observed mapping
// so discovery does not invent another show ID or forget subscription state.
type PodcastFeedAliasesClient interface {
	GetPodcastFeedAlias(context.Context, string) (string, error)
	SavePodcastFeedAlias(context.Context, string, string) error
}

func feedAliasURL(raw string) string {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	return parsed.String()
}
func (c *sqliteClient) GetPodcastFeedAlias(ctx context.Context, feedURL string) (id string, err error) {
	err = c.db.QueryRowContext(ctx, "SELECT channel_id FROM podcast_feed_aliases WHERE feed_url=?", feedAliasURL(feedURL)).Scan(&id)
	return
}
func (c *sqliteClient) SavePodcastFeedAlias(ctx context.Context, feedURL, channelID string) error {
	_, err := c.db.ExecContext(ctx, "INSERT INTO podcast_feed_aliases(feed_url,channel_id) VALUES(?,?) ON CONFLICT(feed_url) DO UPDATE SET channel_id=excluded.channel_id", feedAliasURL(feedURL), channelID)
	return err
}
