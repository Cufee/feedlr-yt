package background

import (
	"sync"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
)

func CacheAllChannelsWithVideos() error {
	channels, err := database.DefaultClient.GetAllChannelsWithSubscriptions()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var limiter = make(chan int, 3)
	var errChan = make(chan error, len(channels))
	for _, c := range channels {
		wg.Add(1)
		go func(c models.Channel) {
			defer wg.Done()

			limiter <- 1
			defer func() { <-limiter }()

			_, err := logic.CacheChannelVideos(c.ExternalID)
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
