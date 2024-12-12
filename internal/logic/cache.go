package logic

import (
	"context"

	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/friendsofgo/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

/*
Cache recent videos for each channel to the database
*/
func CacheChannelVideos(ctx context.Context, db database.Client, limit int, channelIds ...string) ([]*models.Video, error) {
	if len(channelIds) < 1 {
		return nil, errors.New("at least 1 channel id is required")
	}

	var updates []*models.Video
	var group errgroup.Group
	group.SetLimit(1)

	for _, c := range channelIds {
		channelID := c
		group.Go(func() error {
			cctx, ccancel := context.WithTimeout(ctx, time.Second*30)
			defer ccancel()

			channel, _, err := CacheChannel(cctx, db, channelID)
			if err != nil {
				return err
			}

			dctx, dcancel := context.WithTimeout(ctx, time.Second*5)
			defer dcancel()

			existingVideos, err := db.FindVideos(
				dctx,
				database.Video.Limit(24),
				// we should not skip failed videos
				database.Video.Channel(channelID), database.Video.TypeNot(string(youtube.VideoTypeFailed)),
				database.Video.Select(models.VideoColumns.ID, models.VideoColumns.ChannelID, models.VideoColumns.Type),
			)
			if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
				return errors.Wrap(err, "db#FindVideos")
			}

			var existingIDs []string
			for _, v := range existingVideos {
				existingIDs = append(existingIDs, v.ID)
			}

			// since we make a list of videos to skip,
			// we can check back a little more to effectively retry failed ones
			videosSince := time.Now().Add(-2 * (time.Since(channel.FeedUpdatedAt) + time.Hour))

			recentVideos, err := youtube.DefaultClient.GetPlaylistVideos(channel.UploadsPlaylistID, videosSince, limit, existingIDs...)
			if err != nil {
				return errors.Wrap(err, "youtube#GetPlaylistVideos")
			}

			var updated bool
			for _, video := range recentVideos {
				updates = append(updates, &models.Video{
					ChannelID:   c,
					ID:          video.ID,
					Type:        string(video.Type),
					Title:       video.Title,
					Duration:    int64(video.Duration),
					Description: video.Description,
					PublishedAt: video.PublishedAt,
					Private:     video.Type == youtube.VideoTypePrivate,
				})
				updated = true
			}
			if updated {
				uctx, ucancel := context.WithTimeout(ctx, time.Second)
				defer ucancel()
				return db.SetChannelFeedUpdatedAt(uctx, channelID, time.Now())
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

	err := db.UpsertVideos(ctx, updates...)
	if err != nil {
		return nil, errors.Wrap(err, "db#UpsertVideos")
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
		return nil, false, errors.Wrap(err, "youtube#GetChannel")
	}

	uploadsPlaylist, err := youtube.DefaultClient.GetChannelUploadPlaylistID(channelID)
	if err != nil {
		return nil, false, errors.Wrap(err, "youtube#GetChannelUploadPlaylistID")
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
		return nil, false, errors.Wrap(err, "db#UpsertChannel")
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

	update := &models.Video{
		ChannelID:   video.ChannelID,
		ID:          video.ID,
		Type:        string(video.Type),
		Title:       video.Title,
		Duration:    int64(video.Duration),
		Description: video.Description,
		PublishedAt: video.PublishedAt,
		Private:     video.Type == youtube.VideoTypePrivate,
	}

	err = db.UpsertVideos(ctx, update)
	if err != nil {
		return err
	}
	return nil
}
