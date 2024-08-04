package background

import (
	"context"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
)

type databaseClient interface {
	database.ChannelsClient
	database.VideosClient
}

func CacheAllChannelsWithVideos(db databaseClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	channels, err := db.GetChannelsWithSubscriptions(ctx)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var limiter = make(chan int, 3)
	var errChan = make(chan error, len(channels))
	for _, c := range channels {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			limiter <- 1
			defer func() { <-limiter }()

			_, err := logic.CacheChannelVideos(context.Background(), db, id)
			if err != nil {
				errChan <- err
			}
		}(c.ID)
	}
	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}
