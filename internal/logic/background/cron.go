package background

import (
	"log"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/go-co-op/gocron"
)

func StartCronTasks(db database.Client) (*gocron.Scheduler, error) {
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

	s.StartAsync()
	return s, nil
}
