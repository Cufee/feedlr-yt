package background

import (
	"sync"

	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/logic"
	"github.com/byvko-dev/youtube-app/prisma/db"
)

func CacheAllChannelsWithVideos() error {
	channels, err := database.C.GetAllChannels()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var errChan = make(chan error, len(channels))
	for _, c := range channels {
		wg.Add(1)
		go func(c db.ChannelModel) {
			defer wg.Done()
			err := logic.CacheChannelVideos(c.ID)
			if err != nil {
				errChan <- err
			}
		}(c)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}
