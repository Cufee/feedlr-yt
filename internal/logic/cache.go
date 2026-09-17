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
			if err != nil && !database.IsErrNotFound(err) {
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
	if youtube.DefaultClient == nil {
		err := errors.New("youtube client is unavailable")
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, err
	}
	return cacheChannel(ctx, db, channelID, youtube.DefaultClient.GetChannelPage)
}

func cacheChannel(
	ctx context.Context,
	db database.ChannelsClient,
	channelID string,
	fetch func(context.Context, string) (*youtube.Channel, error),
) (*models.Channel, bool, error) {
	uploadsPlaylistID, err := youtube.ChannelUploadsPlaylistID(channelID)
	if err != nil {
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, err
	}

	dctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	existing, err := db.GetChannel(dctx, channelID)
	if err != nil && !database.IsErrNotFound(err) {
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, errors.Wrap(err, "db#GetChannel")
	}
	if err != nil {
		existing = nil
	}

	if existing != nil && time.Since(existing.UpdatedAt) < 7*24*time.Hour {
		if existing.UploadsPlaylistID != uploadsPlaylistID {
			existing.UploadsPlaylistID = uploadsPlaylistID
			if err := upsertCachedChannel(ctx, db, existing); err != nil {
				metrics.ObserveVideoRefresh("cache_channel", err)
				return nil, false, err
			}
		}
		metrics.ObserveVideoRefresh("cache_channel", nil)
		return existing, true, nil
	}

	channel, err := fetch(ctx, channelID)
	if err == nil && (channel == nil || channel.ID != channelID || channel.Title == "") {
		err = errors.New("public channel metadata is incomplete")
	}
	if err != nil {
		if existing != nil && ctx.Err() == nil {
			existing.UploadsPlaylistID = uploadsPlaylistID
			log.Warn().Err(err).Str("channelID", channelID).Msg("using stale channel after public metadata fetch failed")
			metrics.ObserveVideoRefresh("cache_channel_stale_fallback", nil)
			return existing, true, nil
		}
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, errors.Wrap(err, "youtube#GetChannelPage")
	}
	record := existing
	if record == nil {
		record = &models.Channel{ID: channelID}
	}
	record.Title = channel.Title
	if channel.Description != "" {
		record.Description = channel.Description
	}
	if channel.Thumbnail != "" {
		record.Thumbnail = channel.Thumbnail
	}
	record.UploadsPlaylistID = uploadsPlaylistID

	if err := upsertCachedChannel(ctx, db, record); err != nil {
		metrics.ObserveVideoRefresh("cache_channel", err)
		return nil, false, err
	}

	metrics.ObserveVideoRefresh("cache_channel", nil)
	// Return cached=true when refreshing an existing channel (only metadata changed)
	return record, existing != nil, nil
}

func upsertCachedChannel(ctx context.Context, db database.ChannelsClient, channel *models.Channel) error {
	uctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := db.UpsertChannel(uctx, channel); err != nil {
		return errors.Wrap(err, "db#UpsertChannel")
	}
	return nil
}

func RefreshVideoCache(ctx context.Context, db database.Client, videoID string) {
	current, err := db.GetVideoByID(ctx, videoID)
	if err == nil && current != nil && current.Type == string(youtube.VideoTypePodcastEpisode) {
		// Podcast episodes refresh through their feed; nothing to do here.
		return
	}
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
	update := &models.Video{
		ChannelID:   video.ChannelID,
		ID:          video.ID,
		Type:        string(video.Type),
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

	// Resolve title AFTER type guard so the corrected type is used
	currentTitle := ""
	if current != nil {
		currentTitle = current.Title
	}
	update.Title = resolveVideoTitle(video.Title, currentTitle, video.ID, youtube.VideoType(update.Type))

	// Ensure channel exists before upserting video (FK constraint on channel_id)
	if _, _, err := CacheChannel(ctx, db, video.ChannelID); err != nil {
		log.Warn().Err(err).Str("videoID", videoID).Str("channelID", video.ChannelID).Msg("failed to cache channel before video upsert")
		return
	}

	if err := db.UpsertVideos(ctx, update); err != nil {
		metrics.ObserveVideoRefresh("refresh_video_cache", err)
		log.Warn().Err(err).Str("videoID", videoID).Msg("failed to upsert video during cache refresh")
		return
	}

	metrics.ObserveVideoRefresh("refresh_video_cache", nil)
	metrics.AddVideoRefreshItems("refresh_video_cache", 1)
}
