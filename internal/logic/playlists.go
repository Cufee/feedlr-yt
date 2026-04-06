package logic

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/friendsofgo/errors"
	"github.com/lucsky/cuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
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

// --- User Playlist Management ---

var ErrSyncTooSoon = errors.New("playlist was synced recently")

var slugRegex = regexp.MustCompile(`[^a-z0-9-]+`)

func slugFromName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	slug := slugRegex.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "playlist"
	}
	// Truncate to reasonable length
	r := []rune(slug)
	if len(r) > 40 {
		slug = string(r[:40])
	}
	suffix := cuid.New()
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return slug + "-" + suffix
}

func CreateUserPlaylist(ctx context.Context, db database.PlaylistsClient, userID, name, description string) (*models.Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("playlist name is required")
	}

	playlist := &models.Playlist{
		UserID:      userID,
		Slug:        slugFromName(name),
		Name:        name,
		Description: strings.TrimSpace(description),
		System:      false,
	}
	if err := db.CreatePlaylist(ctx, playlist); err != nil {
		return nil, errors.Wrap(err, "failed to create playlist")
	}
	return playlist, nil
}

func CreateImportedPlaylist(ctx context.Context, db database.PlaylistsClient, userID, name, description, youtubePlaylistID string) (*models.Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Imported Playlist"
	}

	playlist := &models.Playlist{
		UserID:            userID,
		Slug:              slugFromName(name),
		Name:              name,
		Description:       strings.TrimSpace(description),
		System:            false,
		YoutubePlaylistID: null.StringFrom(youtubePlaylistID),
	}
	if err := db.CreatePlaylist(ctx, playlist); err != nil {
		return nil, errors.Wrap(err, "failed to create imported playlist")
	}
	return playlist, nil
}

func UpdateUserPlaylist(ctx context.Context, db database.PlaylistsClient, userID, playlistID, name, description string) error {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return errors.New("cannot modify this playlist")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("playlist name is required")
	}

	playlist.Name = name
	playlist.Description = strings.TrimSpace(description)
	playlist.UpdatedAt = time.Now()
	return db.UpdatePlaylist(ctx, playlist)
}

func DeleteUserPlaylist(ctx context.Context, db database.PlaylistsClient, userID, playlistID string) error {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return errors.New("cannot delete this playlist")
	}
	return db.DeletePlaylist(ctx, playlistID)
}

func GetUserPlaylistsProps(ctx context.Context, db interface {
	database.PlaylistsClient
	database.ViewsClient
}, userID string) ([]types.PlaylistProps, error) {
	playlists, err := db.GetUserPlaylists(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user playlists")
	}

	result := make([]types.PlaylistProps, 0, len(playlists))
	for _, p := range playlists {
		count, _ := db.GetPlaylistItemCount(ctx, p.ID)
		thumbVideoID, _ := db.GetPlaylistFirstVideoID(ctx, p.ID)

		progress := 0
		if count > 0 {
			progress = computePlaylistProgress(ctx, db, p.ID, userID)
		}

		result = append(result, types.PlaylistModelToProps(p, int(count), progress, thumbVideoID))
	}
	return result, nil
}

func computePlaylistProgress(ctx context.Context, db interface {
	database.PlaylistsClient
	database.ViewsClient
}, playlistID, userID string) int {
	items, err := db.GetPlaylistItems(ctx, playlistID)
	if err != nil || len(items) == 0 {
		return 0
	}

	videoIDs := make([]string, len(items))
	for i, item := range items {
		videoIDs[i] = item.VideoID
	}

	views, err := GetUserViews(ctx, db, userID, videoIDs...)
	if err != nil {
		return 0
	}

	watched := 0
	for _, id := range videoIDs {
		if v, ok := views[id]; ok && v.Progress > 0 {
			watched++
		}
	}

	return (watched * 100) / len(items)
}

type PlaylistPageProps struct {
	Playlist types.PlaylistProps
	New      []types.VideoProps
	Watched  []types.VideoProps
}

func GetPlaylistPageProps(ctx context.Context, db interface {
	database.PlaylistsClient
	database.ViewsClient
}, userID, playlistID string) (*PlaylistPageProps, error) {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID {
		return nil, errors.New("playlist not found")
	}

	items, err := db.GetPlaylistItems(ctx, playlistID,
		database.PlaylistItem.OrderByPosition(),
		database.PlaylistItem.WithVideo(),
		database.PlaylistItem.WithChannel(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get playlist items")
	}

	videoIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.R != nil && item.R.Video != nil {
			videoIDs = append(videoIDs, item.VideoID)
		}
	}

	views, err := GetUserViews(ctx, db, userID, videoIDs...)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "failed to get user views")
	}

	count, _ := db.GetPlaylistItemCount(ctx, playlistID)
	thumbVideoID, _ := db.GetPlaylistFirstVideoID(ctx, playlistID)
	progress := 0
	if count > 0 {
		progress = computePlaylistProgress(ctx, db, playlistID, userID)
	}

	props := &PlaylistPageProps{
		Playlist: types.PlaylistModelToProps(playlist, int(count), progress, thumbVideoID),
	}

	for _, item := range items {
		if item.R == nil || item.R.Video == nil {
			continue
		}
		video := item.R.Video
		var channelProps types.ChannelProps
		if video.R != nil && video.R.Channel != nil {
			channelProps = types.ChannelModelToProps(video.R.Channel)
		}
		vp := types.VideoModelToProps(video, channelProps)

		if view, ok := views[video.ID]; ok {
			vp.Progress = int(view.Progress)
			vp.Hidden = view.Hidden.Bool
		}

		if vp.Hidden {
			continue
		}

		if vp.Progress > 0 {
			props.Watched = append(props.Watched, vp)
		} else {
			props.New = append(props.New, vp)
		}
	}

	return props, nil
}

func AddVideoToPlaylist(ctx context.Context, db database.PlaylistsClient, userID, playlistID, videoID string) error {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return errors.New("cannot modify this playlist")
	}

	already, err := db.IsVideoInPlaylist(ctx, playlistID, videoID)
	if err != nil {
		return errors.Wrap(err, "failed to check playlist membership")
	}
	if already {
		return nil
	}

	return db.AddPlaylistItem(ctx, playlistID, videoID)
}

func RemoveVideoFromPlaylist(ctx context.Context, db database.PlaylistsClient, userID, playlistID, videoID string) error {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return errors.New("cannot modify this playlist")
	}
	return db.RemovePlaylistItem(ctx, playlistID, videoID)
}

func MovePlaylistItem(ctx context.Context, db database.PlaylistsClient, userID, playlistID, videoID, direction string) error {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return errors.New("cannot modify this playlist")
	}
	return db.SwapPlaylistItemPositions(ctx, playlistID, videoID, direction)
}

func GetVideoPlaylistMembership(ctx context.Context, db database.PlaylistsClient, userID, videoID string) (map[string]bool, error) {
	playlists, err := db.GetUserPlaylists(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user playlists")
	}

	result := make(map[string]bool, len(playlists))
	for _, p := range playlists {
		inPlaylist, err := db.IsVideoInPlaylist(ctx, p.ID, videoID)
		if err != nil {
			continue
		}
		if inPlaylist {
			result[p.ID] = true
		}
	}
	return result, nil
}

func ImportYouTubePlaylist(ctx context.Context, db database.Client, userID, youtubePlaylistID string) (string, error) {
	title, description, err := youtube.DefaultClient.GetPlaylistMetadata(youtubePlaylistID)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch playlist metadata from YouTube")
	}

	// Clean description for storage
	description = cleanDescription(description)

	playlist, err := CreateImportedPlaylist(ctx, db, userID, title, description, youtubePlaylistID)
	if err != nil {
		return "", err
	}

	videoIDs, err := youtube.DefaultClient.GetAllPlaylistVideoIDs(youtubePlaylistID, 500)
	if err != nil {
		return playlist.ID, errors.Wrap(err, "failed to fetch playlist videos from YouTube")
	}

	var group errgroup.Group
	group.SetLimit(5)

	for idx, vid := range videoIDs {
		videoID := vid
		position := idx
		group.Go(func() error {
			// Cache the video in our DB
			RefreshVideoCache(ctx, db, videoID)
			// Only add if the video exists in the DB (FK constraint)
			if _, err := db.GetVideoByID(ctx, videoID); err != nil {
				return nil // skip videos that couldn't be cached
			}
			return db.AddPlaylistItemAtPosition(ctx, playlist.ID, videoID, position)
		})
	}

	if err := group.Wait(); err != nil {
		log.Warn().Err(err).Str("playlistID", playlist.ID).Msg("some videos failed to import")
	}

	return playlist.ID, nil
}

func SyncYouTubePlaylist(ctx context.Context, db database.Client, userID, playlistID string) (int, error) {
	playlist, err := db.GetPlaylistByID(ctx, playlistID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get playlist")
	}
	if playlist.UserID != userID || playlist.System {
		return 0, errors.New("cannot sync this playlist")
	}
	if !playlist.YoutubePlaylistID.Valid || playlist.YoutubePlaylistID.String == "" {
		return 0, errors.New("playlist is not imported from YouTube")
	}

	// Skip cooldown if playlist is empty (e.g., failed initial import)
	itemCount, _ := db.GetPlaylistItemCount(ctx, playlistID)
	if itemCount > 0 && time.Since(playlist.UpdatedAt) < time.Hour {
		return 0, ErrSyncTooSoon
	}

	// Get current video IDs in playlist
	existingItems, err := db.GetPlaylistItems(ctx, playlistID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get existing items")
	}
	existingIDs := make(map[string]bool, len(existingItems))
	for _, item := range existingItems {
		existingIDs[item.VideoID] = true
	}

	// Fetch from YouTube
	ytVideoIDs, err := youtube.DefaultClient.GetAllPlaylistVideoIDs(playlist.YoutubePlaylistID.String, 500)
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch YouTube playlist")
	}

	// Get current max position
	maxPos, err := db.GetMaxPlaylistItemPosition(ctx, playlistID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get max position")
	}

	added := 0
	var group errgroup.Group
	group.SetLimit(5)

	for _, vid := range ytVideoIDs {
		videoID := vid
		if existingIDs[videoID] {
			continue
		}

		maxPos++
		pos := maxPos
		added++

		group.Go(func() error {
			RefreshVideoCache(ctx, db, videoID)
			// Only add if the video exists in the DB (FK constraint)
			if _, err := db.GetVideoByID(ctx, videoID); err != nil {
				return nil // skip videos that couldn't be cached
			}
			return db.AddPlaylistItemAtPosition(ctx, playlistID, videoID, pos)
		})
	}

	if err := group.Wait(); err != nil {
		log.Warn().Err(err).Str("playlistID", playlistID).Int("added", added).Msg("some videos failed during sync")
	}

	// Bump updated_at
	playlist.UpdatedAt = time.Now()
	_ = db.UpdatePlaylist(ctx, playlist)

	return added, nil
}

func cleanDescription(s string) string {
	s = strings.TrimSpace(s)
	// Limit to reasonable length
	r := []rune(s)
	if len(r) > 500 {
		return string(r[:500])
	}
	// Remove control characters except newlines
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
