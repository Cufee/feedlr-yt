package logic

import (
	"context"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/friendsofgo/errors"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

/*
Cache recent videos for each channel to the database
*/
func CacheChannelVideos(ctx context.Context, db database.Client, limit int, channelIds ...string) ([]*models.Video, error) {
	if len(channelIds) < 1 {
		err := errors.New("at least 1 channel id is required")
		metrics.ObserveVideoRefresh("cache_channel_videos", err)
		return nil, err
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
				title := resolveVideoTitle(video.Title, "", video.ID, video.Type)
				updates = append(updates, &models.Video{
					ChannelID:   c,
					ID:          video.ID,
					Type:        string(video.Type),
					Title:       title,
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
		metrics.ObserveVideoRefresh("cache_channel_videos", err)
		return nil, err
	}
	if len(updates) == 0 {
		metrics.ObserveVideoRefresh("cache_channel_videos", nil)
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	err := db.UpsertVideos(ctx, updates...)
	if err != nil {
		metrics.ObserveVideoRefresh("cache_channel_videos", err)
		return nil, errors.Wrap(err, "db#UpsertVideos")
	}

	metrics.ObserveVideoRefresh("cache_channel_videos", nil)
	metrics.AddVideoRefreshItems("cache_channel_videos", len(updates))
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
		metrics.ObserveVideoRefresh("cache_channel", nil)
		return existing, true, nil
	}

	channel, err := youtube.DefaultClient.GetChannel(channelID)
	if err != nil {
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, errors.Wrap(err, "youtube#GetChannel")
	}

	uploadsPlaylist, err := youtube.DefaultClient.GetChannelUploadPlaylistID(channelID)
	if err != nil {
		metrics.ObserveVideoRefresh("cache_channel", err)
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
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, errors.Wrap(err, "db#UpsertChannel")
	}

	metrics.ObserveVideoRefresh("cache_channel", nil)
	return record, false, nil
}

func RefreshVideoCache(ctx context.Context, db database.Client, videoID string) {
	current, err := db.GetVideoByID(ctx, videoID)
	if err != nil && !database.IsErrNotFound(err) {
		metrics.ObserveVideoRefresh("refresh_video_cache", err)
		log.Warn().Err(err).Str("videoID", videoID).Msg("failed to get video for cache refresh")
		return
	}

	var staleThreshold time.Duration
	if current != nil {
		switch current.Type {
		case string(youtube.VideoTypeLiveStream), string(youtube.VideoTypeUpcomingStream), string(youtube.VideoTypeStreamRecording):
			staleThreshold = time.Hour
		default:
			staleThreshold = 6 * time.Hour
		}

		if time.Since(current.UpdatedAt) < staleThreshold {
			metrics.ObserveVideoRefresh("refresh_video_cache_skip_fresh", nil)
			return
		}

		if err := db.TouchVideoUpdatedAt(ctx, videoID); err != nil {
			metrics.ObserveVideoRefresh("refresh_video_cache", err)
			log.Warn().Err(err).Str("videoID", videoID).Msg("failed to touch video timestamp")
			return
		}
	}

	video, err := youtube.DefaultClient.GetVideoDetailsByID(videoID)
	if err != nil {
		metrics.ObserveVideoRefresh("refresh_video_cache", err)
		log.Warn().Err(err).Str("videoID", videoID).Msg("failed to fetch video details for cache refresh")
		return
	}

	if current != nil {
		video.ChannelID = current.ChannelID
	}
	if video.ChannelID == "" {
		metrics.ObserveVideoRefresh("refresh_video_cache", errors.New("missing_channel_id"))
		log.Warn().Str("videoID", videoID).Msg("cannot refresh uncached private video without channel id")
		return
	}
	currentTitle := ""
	if current != nil {
		currentTitle = current.Title
	}
	title := resolveVideoTitle(video.Title, currentTitle, video.ID, video.Type)

	update := &models.Video{
		ChannelID:   video.ChannelID,
		ID:          video.ID,
		Type:        string(video.Type),
		Title:       title,
		Duration:    int64(video.Duration),
		Description: video.Description,
		PublishedAt: video.PublishedAt,
		Private:     video.Type == youtube.VideoTypePrivate,
	}

	// Guard against overwriting good cached data with degraded API responses
	if current != nil {
		if current.Duration > 0 && update.Duration == 0 {
			update.Duration = current.Duration
		}
		if current.Type != string(youtube.VideoTypeFailed) {
			if update.Type == string(youtube.VideoTypeFailed) {
				update.Type = current.Type
			}
			if current.Type == string(youtube.VideoTypeShort) && update.Type == string(youtube.VideoTypeVideo) {
				update.Type = current.Type
			}
		}
	}

	if err := db.UpsertVideos(ctx, update); err != nil {
		metrics.ObserveVideoRefresh("refresh_video_cache", err)
		log.Warn().Err(err).Str("videoID", videoID).Msg("failed to upsert video during cache refresh")
		return
	}

	if _, _, err := CacheChannel(ctx, db, video.ChannelID); err != nil {
		metrics.ObserveVideoRefresh("refresh_video_cache_channel", err)
		log.Warn().Err(err).Str("videoID", videoID).Str("channelID", video.ChannelID).Msg("failed to cache channel during video refresh")
	}
	metrics.ObserveVideoRefresh("refresh_video_cache", nil)
	metrics.AddVideoRefreshItems("refresh_video_cache", 1)
}
