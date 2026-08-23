package database

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type PodcastShowsClient interface {
	GetPodcastShow(ctx context.Context, channelID string) (*models.PodcastShow, error)
	GetPodcastShowByFeedURL(ctx context.Context, feedURL string) (*models.PodcastShow, error)
	SubscribedPodcastShowChannelIDs(ctx context.Context) ([]string, error)
	SubscribedPodcastShowFeedURLs(ctx context.Context) (map[string]string, error)
	UpsertPodcastShow(ctx context.Context, show *models.PodcastShow) error
}

func (c *sqliteClient) GetPodcastShow(ctx context.Context, channelID string) (*models.PodcastShow, error) {
	return models.FindPodcastShow(ctx, c.db, channelID)
}

func (c *sqliteClient) GetPodcastShowByFeedURL(ctx context.Context, feedURL string) (*models.PodcastShow, error) {
	return models.PodcastShows(models.PodcastShowWhere.FeedURL.EQ(feedURL)).One(ctx, c.db)
}

// SubscribedPodcastShowChannelIDs returns channel IDs of podcast shows that
// have at least one subscription.
func (c *sqliteClient) SubscribedPodcastShowChannelIDs(ctx context.Context) ([]string, error) {
	shows, err := models.PodcastShows(
		qm.Select(models.PodcastShowColumns.ChannelID),
		qm.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = "+models.TableNames.PodcastShows+".channel_id)"),
	).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(shows))
	for _, show := range shows {
		ids = append(ids, show.ChannelID)
	}
	return ids, nil
}

// SubscribedPodcastShowFeedURLs maps channel IDs to feed URLs for subscribed
// podcast shows.
func (c *sqliteClient) SubscribedPodcastShowFeedURLs(ctx context.Context) (map[string]string, error) {
	shows, err := models.PodcastShows(
		qm.Select(models.PodcastShowColumns.ChannelID, models.PodcastShowColumns.FeedURL),
		qm.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.channel_id = "+models.TableNames.PodcastShows+".channel_id)"),
	).All(ctx, c.db)
	if err != nil {
		return nil, err
	}

	urls := make(map[string]string, len(shows))
	for _, show := range shows {
		urls[show.ChannelID] = show.FeedURL
	}
	return urls, nil
}

func (c *sqliteClient) UpsertPodcastShow(ctx context.Context, show *models.PodcastShow) error {
	return show.Upsert(ctx, c.db, true, []string{models.PodcastShowColumns.ChannelID}, boil.Blacklist(models.PodcastShowColumns.CreatedAt), boil.Infer())
}
