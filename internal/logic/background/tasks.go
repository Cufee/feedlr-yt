package background

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"golang.org/x/sync/errgroup"
)

func CacheAllChannelsWithVideos(db database.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	channels, err := db.GetChannelsForUpdate(ctx)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		return nil
	}

	var group errgroup.Group
	group.SetLimit(3)

	for _, c := range channels {
		id := c
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel()

			_, err := logic.CacheChannelVideos(ctx, db, 12, id)
			if err != nil {
				return err
			}

			return nil
		})
	}

	return group.Wait()
}
