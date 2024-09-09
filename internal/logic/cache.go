package logic

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/ssoroka/slice"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

/*
Cache recent videos for each channel to the database
*/
func CacheChannelVideos(ctx context.Context, db database.Client, limit int, channelIds ...string) ([]*models.Video, error) {
	var updates []*models.Video

	var group errgroup.Group
	group.SetLimit(1)

	for _, c := range channelIds {
		group.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()

			channel, _, err := CacheChannel(ctx, db, c)
			if err != nil {
				return err
			}

			recentVideos, err := youtube.DefaultClient.GetPlaylistVideos(channel.UploadsPlaylistID, limit)
			if err != nil {
				return errors.Join(errors.New("CacheChannelVideos.youtube.C.GetChannelVideos"), err)
			}

			existingVideos, err := db.FindVideos(ctx, database.Video.Channel(c))
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				return errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.GetVideosByChannelID"), err)
			}

			var existingIDs []string
			for _, v := range existingVideos {
				existingIDs = append(existingIDs, v.ID)
			}

			updated := false
			for _, video := range recentVideos {
				if slice.Contains(existingIDs, video.ID) {
					continue
				}

				publishedAt, err := time.Parse(time.RFC3339, video.PublishedAt)
				if err != nil {
					log.Printf("Error parsing publishedAt %v", err)
				}
				updates = append(updates, &models.Video{
					ChannelID:   c,
					ID:          video.ID,
					Type:        string(video.Type),
					Title:       video.Title,
					Duration:    int64(video.Duration),
					Description: video.Description,
					PublishedAt: publishedAt,
					Private:     video.Type == youtube.VideoTypePrivate,
				})
				updated = true
			}

			if updated {
				return db.SetChannelFeedUpdatedAt(ctx, channel.ID, time.Now())
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	slices.SortFunc(updates, func(a, b *models.Video) int {
		return a.PublishedAt.Compare(b.PublishedAt)
	})
	err := db.UpsertVideos(ctx, updates...)
	if err != nil {
		return nil, errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.InsertChannelVideos"), err)
	}

	return updates, nil
}

/*
Saves the channel to the database if it doesn't exist already and returns the channel model
*/
func CacheChannel(ctx context.Context, db database.ChannelsClient, channelID string) (*models.Channel, bool, error) {
	dctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	existing, err := db.GetChannel(dctx, channelID)
	if err == nil && existing.UploadsPlaylistID != "" {
		return existing, true, nil
	}

	channel, err := youtube.DefaultClient.GetChannel(channelID)
	if err != nil {
		return nil, false, errors.Join(errors.New("CacheChannel.youtube.C.GetChannel"), err)
	}

	uploadsPlaylist, err := youtube.DefaultClient.GetChannelUploadPlaylistID(channelID)
	if err != nil {
		return nil, false, errors.Join(errors.New("youtube#GetChannelUploadPlaylistID"), err)
	}

	record := &models.Channel{
		ID:                channel.ID,
		Title:             channel.Title,
		Description:       channel.Description,
		Thumbnail:         channel.Thumbnail,
		UploadsPlaylistID: uploadsPlaylist,
	}

	uctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err = db.UpsertChannel(uctx, record)
	if err != nil {
		return nil, false, errors.Join(errors.New("CacheChannel.database.DefaultClient.NewChannel"), err)
	}

	return record, false, nil
}

func UpdateChannelVideoCache(ctx context.Context, db database.Client, videoID string) error {
	current, err := db.GetVideoByID(ctx, videoID)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	if current != nil && time.Since(current.UpdatedAt) < time.Hour {
		return nil
	}

	video, err := youtube.DefaultClient.GetVideoDetailsByID(videoID)
	if err != nil {
		return err
	}
	_, _, err = CacheChannel(ctx, db, video.ChannelID)
	if err != nil {
		return err
	}

	publishedAt, _ := time.Parse(time.RFC3339, video.PublishedAt)
	update := &models.Video{
		ChannelID:   video.ChannelID,
		ID:          video.ID,
		Type:        string(video.Type),
		Title:       video.Title,
		Duration:    int64(video.Duration),
		Description: video.Description,
		PublishedAt: publishedAt,
		Private:     video.Type == youtube.VideoTypePrivate,
	}

	err = db.UpsertVideos(ctx, update)
	if err != nil {
		return err
	}
	return nil
}
