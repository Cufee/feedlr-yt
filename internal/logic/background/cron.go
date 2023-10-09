package background

import (
	"log"
	"time"

	"github.com/go-co-op/gocron"
)

func StartCronTasks() *gocron.Scheduler {
	s := gocron.NewScheduler(time.UTC)
	_, err := s.Every(1).Hour().WaitForSchedule().Do(func() {
		log.Print("Caching all channels with videos")
		err := CacheAllChannelsWithVideos()
		if err != nil {
			log.Printf("CacheAllChannelsWithVideos: %v", err)
		}
		log.Print("Done caching all channels with videos")
	})
	if err != nil {
		panic(err)
	}

	s.StartAsync()
	return s
}
