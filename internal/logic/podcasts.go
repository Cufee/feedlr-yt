package logic

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/podcastindex"
	"github.com/cufee/feedlr-yt/internal/api/rss"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/pkg/errors"
)

var (
	// ErrPodcastSearchUnavailable is returned when no PodcastIndex API key is configured.
	ErrPodcastSearchUnavailable = errors.New("podcast search is not configured")
	// ErrInvalidFeedURL is returned when a feed URL cannot be parsed or has an unsupported scheme.
	ErrInvalidFeedURL = errors.New("invalid feed url")
)

// SearchPodcasts searches the PodcastIndex catalog and annotates results with
// the user's current subscriptions.
func SearchPodcasts(
	ctx context.Context,
	db database.Client,
	userID string,
	query string,
	limit int,
) ([]types.PodcastSearchResultProps, error) {
	if podcastindex.DefaultClient == nil {
		return nil, ErrPodcastSearchUnavailable
	}

	var wg sync.WaitGroup

	wg.Add(1)
	var podcasts []podcastindex.Podcast
	var searchErr error
	go func() {
		defer wg.Done()
		podcasts, searchErr = podcastindex.DefaultClient.SearchPodcasts(ctx, query, limit)
	}()

	wg.Add(1)
	var subscribedFeedURLs map[string]string
	var subscribedErr error
	go func() {
		defer wg.Done()
		subscribedFeedURLs, subscribedErr = db.SubscribedPodcastShowFeedURLs(ctx)
	}()

	wg.Wait()
	if searchErr != nil {
		return nil, errors.Wrap(searchErr, "failed to search podcasts")
	}
	if subscribedErr != nil {
		return nil, errors.Wrap(subscribedErr, "failed to get user subscriptions")
	}

	props := make([]types.PodcastSearchResultProps, 0, len(podcasts))
	for _, podcast := range podcasts {
		props = append(props, types.PodcastSearchResultProps{
			FeedURL:      podcast.FeedURL,
			Title:        podcast.Title,
			Description:  podcast.Description,
			ArtworkURL:   podcast.ArtworkURL,
			Author:       podcast.Author,
			EpisodeCount: podcast.EpisodeCount,
			Subscribed:   subscribedFeedURLs != nil && feedURLSubscribed(subscribedFeedURLs, podcast.FeedURL),
		})
	}
	return props, nil
}

func feedURLSubscribed(feedURLs map[string]string, feedURL string) bool {
	for _, subscribed := range feedURLs {
		if subscribed == feedURL {
			return true
		}
	}
	return false
}

// NewPodcastSubscription subscribes a user to a podcast feed. The feed is
// fetched immediately so the show and its recent episodes are cached before
// the subscription is created.
func NewPodcastSubscription(ctx context.Context, db database.Client, userID, feedURL string) (*types.ChannelProps, error) {
	parsed, err := parseFeedURL(feedURL)
	if err != nil {
		return nil, err
	}

	fctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	result, err := rss.FetchFeed(fctx, parsed.String(), "", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch feed")
	}

	channelID := rss.ShowID(result.CanonicalURL)

	if sub, err := db.FindSubscription(ctx, userID, channelID); err == nil && sub != nil {
		return podcastChannelProps(channelID, result, true), nil
	} else if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "failed to check subscription")
	}

	if err := upsertPodcastShow(ctx, db, channelID, result.CanonicalURL, result); err != nil {
		return nil, err
	}
	if err := upsertEpisodes(ctx, db, channelID, result.CanonicalURL, result.Episodes, 50); err != nil {
		return nil, err
	}

	if _, err := db.NewSubscription(ctx, userID, channelID); err != nil {
		return nil, errors.Wrap(err, "failed to create subscription")
	}

	return podcastChannelProps(channelID, result, false), nil
}

func parseFeedURL(feedURL string) (*url.URL, error) {
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return nil, errors.Wrap(ErrInvalidFeedURL, feedURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.Wrap(ErrInvalidFeedURL, feedURL)
	}
	return parsed, nil
}

func podcastChannelProps(channelID string, result *rss.FetchResult, subscribed bool) *types.ChannelProps {
	return &types.ChannelProps{
		Channel: youtube.Channel{
			ID:          channelID,
			Title:       result.Show.Title,
			Thumbnail:   result.Show.ImageURL,
			Description: result.Show.Description,
		},
		FeedUpdatedAt: time.Now(),
		IsPodcast:     true,
		VideoFilter:   types.VideoFilterAll,
	}
}

func upsertPodcastShow(ctx context.Context, db database.Client, channelID, canonicalURL string, result *rss.FetchResult) error {
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	record := &models.Channel{
		ID:          channelID,
		Title:       result.Show.Title,
		Description: result.Show.Description,
		Thumbnail:   result.Show.ImageURL,
	}
	if existing, err := db.GetChannel(cctx, channelID); err == nil {
		// Preserve the refresh timestamp across metadata updates.
		record.FeedUpdatedAt = existing.FeedUpdatedAt
	}
	if err := db.UpsertChannel(cctx, record); err != nil {
		return errors.Wrap(err, "failed to upsert channel")
	}

	sctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	show := &models.PodcastShow{
		ChannelID:    channelID,
		FeedURL:      canonicalURL,
		Etag:         null.StringFrom(result.ETag),
		LastModified: null.StringFrom(result.LastModified),
	}
	if err := db.UpsertPodcastShow(sctx, show); err != nil {
		return errors.Wrap(err, "failed to upsert podcast show")
	}
	return nil
}

func upsertEpisodes(ctx context.Context, db database.Client, channelID, feedURL string, episodes []rss.Episode, limit int) error {
	if len(episodes) > limit {
		episodes = episodes[:limit]
	}

	var updates []*models.Video
	for _, episode := range episodes {
		updates = append(updates, &models.Video{
			ID:          rss.EpisodeID(feedURL, episode.GUID),
			ChannelID:   channelID,
			Type:        string(youtube.VideoTypePodcastEpisode),
			Title:       types.NormalizeVideoTitle(episode.Title, youtube.VideoTypePodcastEpisode, episode.GUID),
			Description: episode.Description,
			Duration:    int64(episode.Duration),
			PublishedAt: episode.PublishedAt,
			Private:     false,
			MediaURL:    null.StringFrom(episode.MediaURL),
		})
	}
	if len(updates) == 0 {
		return nil
	}

	uctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := db.UpsertVideos(uctx, updates...); err != nil {
		return errors.Wrap(err, "failed to upsert episodes")
	}
	return nil
}

// CachePodcastEpisodes refreshes the given podcast channels from their RSS
// feeds using conditional GET requests. A 304 response still bumps the
// channel's feed refresh timestamp.
func CachePodcastEpisodes(ctx context.Context, db database.Client, limit int, channelIDs ...string) error {
	var lastErr error
	for _, channelID := range channelIDs {
		if err := cachePodcastShow(ctx, db, limit, channelID); err != nil {
			lastErr = err
		}
	}
	metrics.ObserveVideoRefresh("cache_podcast_episodes", lastErr)
	return lastErr
}

func cachePodcastShow(ctx context.Context, db database.Client, limit int, channelID string) error {
	sctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	show, err := db.GetPodcastShow(sctx, channelID)
	if database.IsErrNotFound(err) {
		return nil // not a podcast channel
	}
	if err != nil {
		return errors.Wrap(err, "db#GetPodcastShow")
	}

	fctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	result, err := rss.FetchFeed(fctx, show.FeedURL, show.Etag.String, show.LastModified.String)
	if err != nil {
		return errors.Wrap(err, "rss#FetchFeed")
	}

	if !result.NotModified {
		if err := upsertPodcastShow(ctx, db, channelID, show.FeedURL, result); err != nil {
			return err
		}
		if err := upsertEpisodes(ctx, db, channelID, show.FeedURL, result.Episodes, limit); err != nil {
			return err
		}
	}

	tctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := db.SetChannelFeedUpdatedAt(tctx, channelID, time.Now()); err != nil {
		return errors.Wrap(err, "db#SetChannelFeedUpdatedAt")
	}
	return nil
}
