package logic

import (
	"context"
	"errors"
	"sync"
	"time"

	ppd "github.com/cufee/feedlr-yt/internal/api/piped"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/ssoroka/slice"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

/*
Saves last few videos for each channel to the database
*/
func CacheChannelVideos(ctx context.Context, db database.VideosClient, piped *ppd.Client, channelIds ...string) ([]*models.Video, error) {
	var updates []*models.Video
	var updatedMx sync.Mutex

	var group errgroup.Group
	group.SetLimit(1)

	for _, c := range channelIds {
		channelID := c
		group.Go(func() error {
			newVideos, err := piped.Channel(ctx, channelID)
			if err != nil {
				return errors.Join(errors.New("CacheChannelVideos.youtube.C.GetChannelVideos"), err)
			}

			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			existingVideos, err := db.FindVideos(ctx, database.Video{}.Channel(channelID))
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				return errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.GetVideosByChannelID"), err)
			}
			var existingIDs []string
			for _, v := range existingVideos {
				existingIDs = append(existingIDs, v.ID)
			}

			updatedMx.Lock()
			defer updatedMx.Unlock()

			for i, stream := range newVideos.RelatedStreams {
				if slice.Contains(existingIDs, stream.VideoID()) || stream.IsShort || i >= 12 {
					continue
				}
				updates = append(updates, &models.Video{
					ChannelID:   channelID,
					ID:          stream.VideoID(),
					Type:        stream.Type,
					Title:       stream.Title,
					Duration:    int64(stream.Duration),
					Description: stream.Description,
					PublishedAt: stream.PublishDate(),
					Private:     false,
				})

			}
			return nil
		})
	}
	err := group.Wait()
	if err != nil {
		return nil, err
	}

	if len(updates) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	err = db.UpsertVideos(ctx, updates...)
	if err != nil {
		return nil, errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.InsertChannelVideos"), err)
	}
	return updates, nil
}

/*
Saves the channel to the database if it doesn't exist already and returns the channel model
*/
func CacheChannel(ctx context.Context, db database.ChannelsClient, piped *ppd.Client, channelID string) (*models.Channel, bool, error) {
	dctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	existing, err := db.GetChannel(dctx, channelID)
	if err == nil {
		return existing, true, nil
	}

	channel, err := piped.Channel(ctx, channelID)
	if err != nil {
		return nil, false, errors.Join(errors.New("CacheChannel.youtube.C.GetChannel"), err)
	}

	record := &models.Channel{
		ID:          channel.ID,
		Title:       channel.Name,
		Description: channel.Description,
		Thumbnail:   channel.Avatar,
	}

	uctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err = db.UpsertChannel(uctx, record)
	if err != nil {
		return nil, false, errors.Join(errors.New("CacheChannel.database.DefaultClient.NewChannel"), err)
	}

	return record, false, nil
}

func UpdateChannelVideoCache(ctx context.Context, db database.Client, piped *ppd.Client, videoID string) error {
	current, err := db.GetVideoByID(ctx, videoID)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	if current != nil && time.Since(current.UpdatedAt) < time.Hour {
		return nil
	}

	video, err := piped.Video(ctx, videoID)
	if err != nil {
		return err
	}
	_, _, err = CacheChannel(ctx, db, piped, video.ChannelID())
	if err != nil {
		return err
	}

	update := &models.Video{
		ChannelID:   video.ChannelID(),
		ID:          video.ID,
		Type:        string(video.Type),
		Title:       video.Title,
		Duration:    int64(video.Duration),
		Description: video.Description,
		PublishedAt: video.PublishDate(),
		Private:     video.Type == "private",
	}

	err = db.UpsertVideos(ctx, update)
	if err != nil {
		return err
	}
	return nil
}
