package logic

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/friendsofgo/errors"
)

const WatchLaterSlug = "watch-later"
const WatchLaterTTLDays = 30

// getCompletionBuffer returns the buffer in seconds for considering a video fully watched
// - Videos over 30 minutes: 5 minutes buffer
// - Videos over 15 minutes: 1 minute buffer
// - Default: 30 seconds buffer
func getCompletionBuffer(durationSeconds int) int {
	switch {
	case durationSeconds > 30*60: // > 30 minutes
		return 5 * 60 // 5 minutes
	case durationSeconds > 15*60: // > 15 minutes
		return 60 // 1 minute
	default:
		return 30 // 30 seconds
	}
}

// RemoveFromWatchLaterIfFullyWatched removes a video from Watch Later playlist
// if the progress indicates it has been fully watched (within the completion buffer)
func RemoveFromWatchLaterIfFullyWatched(ctx context.Context, db interface {
	database.PlaylistsClient
	database.VideosClient
}, userID, videoID string, progress int) error {
	// Get video to check duration
	video, err := db.GetVideoByID(ctx, videoID)
	if err != nil {
		if database.IsErrNotFound(err) {
			return nil // Video not in DB, nothing to do
		}
		return errors.Wrap(err, "failed to get video")
	}

	duration := int(video.Duration)
	if duration <= 0 {
		return nil // Unknown duration, skip
	}

	buffer := getCompletionBuffer(duration)
	if progress+buffer < duration {
		return nil // Not fully watched yet
	}

	// Video is fully watched, check if it's in Watch Later
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err != nil {
		if database.IsErrNotFound(err) {
			return nil // No Watch Later playlist
		}
		return errors.Wrap(err, "failed to get watch later playlist")
	}

	inPlaylist, err := db.IsVideoInPlaylist(ctx, playlist.ID, videoID)
	if err != nil {
		return errors.Wrap(err, "failed to check if video in playlist")
	}

	if !inPlaylist {
		return nil // Not in Watch Later
	}

	// Remove from Watch Later
	err = db.RemovePlaylistItem(ctx, playlist.ID, videoID)
	if err != nil {
		return errors.Wrap(err, "failed to remove from watch later")
	}

	return nil
}

// GetOrCreateWatchLater gets the user's Watch Later playlist, creating it if it doesn't exist
func GetOrCreateWatchLater(ctx context.Context, db database.PlaylistsClient, userID string) (*models.Playlist, error) {
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err == nil {
		return playlist, nil
	}
	if !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "failed to get watch later playlist")
	}

	// Create the playlist
	playlist = database.NewWatchLaterPlaylist(userID)
	err = db.CreatePlaylist(ctx, playlist)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create watch later playlist")
	}

	return playlist, nil
}

// ToggleWatchLater adds or removes a video from the user's Watch Later playlist
// Returns true if the video is now in the playlist, false if removed
func ToggleWatchLater(ctx context.Context, db database.PlaylistsClient, userID, videoID string) (bool, error) {
	playlist, err := GetOrCreateWatchLater(ctx, db, userID)
	if err != nil {
		return false, err
	}

	inPlaylist, err := db.IsVideoInPlaylist(ctx, playlist.ID, videoID)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if video in playlist")
	}

	if inPlaylist {
		err = db.RemovePlaylistItem(ctx, playlist.ID, videoID)
		if err != nil {
			return false, errors.Wrap(err, "failed to remove from watch later")
		}
		return false, nil
	}

	err = db.AddPlaylistItem(ctx, playlist.ID, videoID)
	if err != nil {
		return false, errors.Wrap(err, "failed to add to watch later")
	}
	return true, nil
}

// IsInWatchLater checks if a video is in the user's Watch Later playlist
func IsInWatchLater(ctx context.Context, db database.PlaylistsClient, userID, videoID string) (bool, error) {
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err != nil {
		if database.IsErrNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to get watch later playlist")
	}

	return db.IsVideoInPlaylist(ctx, playlist.ID, videoID)
}

// GetWatchLaterVideos returns videos from the user's Watch Later playlist with pagination
func GetWatchLaterVideos(ctx context.Context, db interface {
	database.PlaylistsClient
	database.ViewsClient
}, userID string, limit, offset int) ([]types.VideoProps, bool, error) {
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err != nil {
		if database.IsErrNotFound(err) {
			return nil, false, nil
		}
		return nil, false, errors.Wrap(err, "failed to get watch later playlist")
	}

	// Fetch limit + 1 to detect hasMore
	items, err := db.GetPlaylistItems(ctx, playlist.ID,
		database.PlaylistItem.Limit(limit+1),
		database.PlaylistItem.Offset(offset),
		database.PlaylistItem.WithVideo(),
		database.PlaylistItem.WithChannel(),
	)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to get playlist items")
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	// Get user progress for all videos
	videoIDs := make([]string, len(items))
	for i, item := range items {
		videoIDs[i] = item.VideoID
	}

	views, err := GetUserViews(ctx, db, userID, videoIDs...)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, false, errors.Wrap(err, "failed to get user views")
	}

	var videos []types.VideoProps
	for _, item := range items {
		if item.R == nil || item.R.Video == nil {
			continue
		}

		video := item.R.Video
		var channelProps types.ChannelProps
		if video.R != nil && video.R.Channel != nil {
			channelProps = types.ChannelModelToProps(video.R.Channel)
		}

		props := types.VideoModelToProps(video, channelProps)
		props.InWatchLater = true

		if view, ok := views[video.ID]; ok {
			props.Progress = int(view.Progress)
			props.Hidden = view.Hidden.Bool
		}

		videos = append(videos, props)
	}

	return videos, hasMore, nil
}

// GetWatchLaterCount returns the number of videos in the user's Watch Later playlist
func GetWatchLaterCount(ctx context.Context, db database.PlaylistsClient, userID string) (int, error) {
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err != nil {
		if database.IsErrNotFound(err) {
			return 0, nil
		}
		return 0, errors.Wrap(err, "failed to get watch later playlist")
	}

	items, err := db.GetPlaylistItems(ctx, playlist.ID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get playlist items")
	}

	return len(items), nil
}

// GetWatchLaterVideoIDs returns a map of video IDs that are in the user's Watch Later playlist
func GetWatchLaterVideoIDs(ctx context.Context, db database.PlaylistsClient, userID string) (map[string]bool, error) {
	playlist, err := db.GetPlaylistBySlug(ctx, userID, WatchLaterSlug)
	if err != nil {
		if database.IsErrNotFound(err) {
			return make(map[string]bool), nil
		}
		return nil, errors.Wrap(err, "failed to get watch later playlist")
	}

	items, err := db.GetPlaylistItems(ctx, playlist.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get playlist items")
	}

	result := make(map[string]bool, len(items))
	for _, item := range items {
		result[item.VideoID] = true
	}

	return result, nil
}
