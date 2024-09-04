package background

import (
	"context"
	"log"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"golang.org/x/sync/errgroup"
)

func CacheAllChannelsWithVideos(db database.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	channels, err := db.GetChannelsForUpdate(ctx)
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		return nil
	}
	log.Printf("Updating %v channels\n", len(channels))

	var group errgroup.Group
	group.SetLimit(3)

	for _, c := range channels {
		id := c
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			videos, err := logic.CacheChannelVideos(ctx, db, id)
			log.Printf("Updated %v videos for %s\n", len(videos), id)
			return err
		})
	}

	return group.Wait()
}
