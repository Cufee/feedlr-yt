package background

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/piped"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"golang.org/x/sync/errgroup"
)

type databaseClient interface {
	database.ChannelsClient
	database.VideosClient
}

func CacheAllChannelsWithVideos(db databaseClient, piped *piped.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	channels, err := db.GetChannelsWithSubscriptions(ctx)
	if err != nil {
		return err
	}

	var group errgroup.Group
	group.SetLimit(10)

	for _, c := range channels {
		id := c.ID
		group.Go(func() error {
			_, err := logic.CacheChannelVideos(context.Background(), db, piped, id)
			return err
		})
	}

	return group.Wait()
}
