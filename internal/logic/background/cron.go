package background

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/go-co-op/gocron"
)

func StartCronTasks(db database.Client, sync *logic.YouTubeSyncService) (*gocron.Scheduler, error) {
	s := gocron.NewScheduler(time.UTC)

	_, err := s.Cron(utils.MustGetEnv("VIDEO_CACHE_UPDATE_CRON")).Do(func() {
		err := CacheAllChannelsWithVideos(db)
		if err != nil {
			log.Printf("CacheAllChannelsWithVideos: %v", err)
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

		err := sync.RunSyncTick(ctx)
		if err != nil {
			log.Printf("RunSyncTick: %v", err)
		}
	})
	if err != nil {
		return nil, err
	}

	s.StartAsync()
	return s, nil
}
