package background

import (
	"log"
	"time"

	"github.com/go-co-op/gocron"
)

func StartCronTasks() {
	s := gocron.NewScheduler(time.UTC)
	_, err := s.Every(30).Minute().Do(func() {
		log.Print("Caching all channels with videos")
		err := CacheAllChannelsWithVideos()
		if err != nil {
			log.Printf("CacheAllChannelsWithVideos: %v", err)
		}
	})
	if err != nil {
		panic(err)
	}

	s.StartBlocking()
}
