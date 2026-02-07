package background

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/go-co-op/gocron"
)

func StartCronTasks(db database.Client, sync *logic.YouTubeSyncService, tvSync *logic.YouTubeTVSyncService) (*gocron.Scheduler, error) {
	s := gocron.NewScheduler(time.UTC)

	_, err := s.Cron(utils.MustGetEnv("VIDEO_CACHE_UPDATE_CRON")).Do(func() {
		runErr := CacheAllChannelsWithVideos(db)
		metrics.ObserveBackgroundTask("cache_all_channels_with_videos", runErr)
		if runErr != nil {
			log.Printf("CacheAllChannelsWithVideos: %v", runErr)
		}
	})
	if err != nil {
		return nil, err
	}

	// Daily cleanup of expired playlist items (runs at 3 AM UTC)
	_, err = s.Cron("0 3 * * *").Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		deleted, err := db.CleanupExpiredPlaylistItems(ctx)
		metrics.ObserveBackgroundTask("cleanup_expired_playlist_items", err)
		if err != nil {
			log.Printf("CleanupExpiredPlaylistItems: %v", err)
		} else if deleted > 0 {
			log.Printf("CleanupExpiredPlaylistItems: deleted %d expired items", deleted)
		}
	})
	if err != nil {
		return nil, err
	}

	playlistCron := os.Getenv("PLAYLIST_SYNC_CRON")
	if playlistCron == "" {
		playlistCron = "*/30 * * * *"
	}

	_, err = s.Cron(playlistCron).Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
		defer cancel()

		runErr := sync.RunSyncTick(ctx)
		metrics.ObserveBackgroundTask("youtube_sync_run_tick", runErr)
		if runErr != nil {
			log.Printf("RunSyncTick: %v", runErr)
		}
	})
	if err != nil {
		return nil, err
	}

	if tvSync != nil {
		_, err = s.Cron("*/1 * * * *").Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			runErr := tvSync.RunConnectionTick(ctx)
			metrics.ObserveBackgroundTask("youtube_tv_sync_connection_tick", runErr)
			if runErr != nil {
				log.Printf("RunConnectionTick: %v", runErr)
			}
		})
		if err != nil {
			return nil, err
		}

		_, err = s.Cron("*/15 * * * *").Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			runErr := tvSync.RunLifecycleTick(ctx)
			metrics.ObserveBackgroundTask("youtube_tv_sync_lifecycle_tick", runErr)
			if runErr != nil {
				log.Printf("RunLifecycleTick: %v", runErr)
			}
		})
		if err != nil {
			return nil, err
		}
	}

	s.StartAsync()
	return s, nil
}
