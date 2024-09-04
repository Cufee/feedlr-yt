package background

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"golang.org/x/sync/errgroup"
)

type databaseClient interface {
	database.ChannelsClient
	database.VideosClient
}

func CacheAllChannelsWithVideos(db databaseClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	channels, err := db.GetChannelsForUpdate(ctx)
	if err != nil {
		return err
	}

	var group errgroup.Group
	group.SetLimit(3)

	for _, c := range channels {
		id := c
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			_, err := logic.CacheChannelVideos(ctx, db, id)
			return err
		})
	}

	return group.Wait()
}
