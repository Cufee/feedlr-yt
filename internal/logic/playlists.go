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
