package background

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"golang.org/x/sync/errgroup"
)

// CacheAllPodcastEpisodes refreshes every subscribed podcast show from its
// RSS feed. Feeds are polled with conditional GET requests, so unchanged
// feeds are a single cheap request each.
func CacheAllPodcastEpisodes(db database.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	channels, err := db.SubscribedPodcastShowChannelIDs(ctx)
	metrics.ObserveVideoRefresh("cache_all_podcasts_fetch_shows", err)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		metrics.ObserveVideoRefresh("cache_all_podcasts", nil)
		return nil
	}

	var group errgroup.Group
	group.SetLimit(10)

	for _, c := range channels {
		id := c
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			err := logic.CachePodcastEpisodes(ctx, db, 25, id)
			metrics.ObserveVideoRefresh("cache_podcast_show", err)
			if err != nil {
				return err
			}
			return nil
		})
	}

	err = group.Wait()
	metrics.ObserveVideoRefresh("cache_all_podcasts", err)
	if err == nil {
		metrics.AddVideoRefreshItems("cache_all_podcasts", len(channels))
	}
	return err
}

func CacheAllChannelsWithVideos(db database.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	channels, err := db.GetChannelsForUpdate(ctx)
	metrics.ObserveVideoRefresh("cache_all_channels_fetch_channels", err)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		metrics.ObserveVideoRefresh("cache_all_channels", nil)
		return nil
	}

	var group errgroup.Group
	group.SetLimit(10)

	for _, c := range channels {
		id := c
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel()

			_, err := logic.CacheChannelVideos(ctx, db, 12, id)
			metrics.ObserveVideoRefresh("cache_channel_videos", err)
			if err != nil {
				return err
			}

			return nil
		})
	}

	err = group.Wait()
	metrics.ObserveVideoRefresh("cache_all_channels", err)
	if err == nil {
		metrics.AddVideoRefreshItems("cache_all_channels", len(channels))
	}
	return err
}
