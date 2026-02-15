package logic

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/friendsofgo/errors"
	"github.com/gofiber/fiber/v2/log"
)

const (
	homeFeedFetchWindow = 96
	homeFeedLimit       = 36
)

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserVideosProps(ctx context.Context, db database.Client, userId string) (*types.UserVideoFeedProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(ctx, db, userId)
	if err != nil {
		return nil, errors.Wrap(err, "GetUserSubscriptionsProps.GetUserSubscribedChannels failed to get user subscribed channels")
	}

	// Get videos for each channel and add them to the props
	channelsMap := make(map[string]types.ChannelProps)
	var channelIds []string
	for _, c := range channels {
		channelsMap[c.ID] = c
		channelIds = append(channelIds, c.ID)
	}

	allVideos, err := GetChannelVideos(ctx, db, homeFeedFetchWindow, channelIds...)
	if err != nil {
		return nil, errors.Wrap(err, "GetUserSubscriptionsProps.GetChannelVideos failed to get channel videos")
	}

	// Apply per-channel video filters and set channel props with VideoFilter
	var filteredVideos []types.VideoProps
	for _, video := range allVideos {
		// Replace video's channel with the one from channelsMap (which has VideoFilter set)
		if channelWithFilter, ok := channelsMap[video.Channel.ID]; ok {
			video.Channel = channelWithFilter
		}
		channelFilter := video.Channel.VideoFilter
		if channelFilter == "" {
			channelFilter = types.VideoFilterAll
		}
		// Use filterVideosByType to check if this video passes the channel's filter
		if len(filterVideosByType([]types.VideoProps{video}, channelFilter)) > 0 {
			filteredVideos = append(filteredVideos, video)
		}
	}

	videoIds := make([]string, len(filteredVideos))
	for i, v := range filteredVideos {
		videoIds[i] = v.ID
	}

	views, err := GetUserViews(ctx, db, userId, videoIds...)
	if err != nil {
		return nil, errors.Wrap(err, "GetUserSubscriptionsProps.GetCompleteUserProgress failed to get user progress")
	}

	// Get watch later video IDs to set InWatchLater field
	watchLaterIDs, _ := GetWatchLaterVideoIDs(ctx, db, userId)

	var feed types.UserVideoFeedProps
	for _, video := range filteredVideos {
		video.InWatchLater = watchLaterIDs[video.ID]
		if v, ok := views[video.ID]; ok {
			if v.Hidden.Bool {
				continue
			}
			video.Progress = int(v.Progress)
			feed.Watched = append(feed.Watched, video)
		} else {
			feed.New = append(feed.New, video)
		}
	}

	if total := len(feed.New) + len(feed.Watched); total > homeFeedLimit {
		remaining := homeFeedLimit
		if len(feed.New) > remaining {
			feed.New = feed.New[:remaining]
			feed.Watched = nil
		} else {
			remaining -= len(feed.New)
			if len(feed.Watched) > remaining {
				feed.Watched = feed.Watched[:remaining]
			}
		}
	}

	return &feed, nil
}

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetRecentVideosProps(ctx context.Context, db interface {
	database.VideosClient
	database.ViewsClient
}, userId string) ([]types.VideoProps, error) {
	views, err := db.GetRecentUserViews(ctx, userId, 24)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "GetCompleteUserProgress.database.DefaultClient.GetAllUserViews failed to get user views")
	}
	if len(views) == 0 {
		return nil, nil
	}

	var videoIDs []string
	progress := make(map[string]*models.View)
	for _, v := range views {
		videoIDs = append(videoIDs, v.VideoID)
		progress[v.VideoID] = v
	}

	videos, err := db.FindVideos(ctx, database.Video.ID(videoIDs...), database.Video.WithChannel())
	if err != nil {
		return nil, errors.Wrap(err, "db#FindVideos")
	}

	var feed []types.VideoProps
	for _, video := range videos {
		v := types.VideoModelToProps(video, types.ChannelModelToProps(video.R.Channel))
		if view, ok := progress[video.ID]; ok {
			v.Progress = int(view.Progress)
		}
		feed = append(feed, v)
	}
	slices.SortFunc(feed, func(a, b types.VideoProps) int {
		var au, bu time.Time
		if v, ok := progress[a.ID]; ok {
			au = v.UpdatedAt
		}
		if v, ok := progress[b.ID]; ok {
			bu = v.UpdatedAt
		}
		return bu.Compare(au)
	})

	return feed, nil
}

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(ctx context.Context, db database.ChannelsClient, limit int, channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	channels, err := db.GetChannels(ctx, database.Channel.ID(channelIds...), database.Channel.WithVideos(limit))
	if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "GetChannelVideos.database.DefaultClient.GetVideosByChannelID failed to get videos")
	}

	var props []types.VideoProps
	for _, channel := range channels {
		c := types.ChannelModelToProps(channel)
		for _, video := range channel.R.Videos {
			props = append(props, types.VideoModelToProps(video, c))
		}
	}

	slices.SortFunc(props, func(a, b types.VideoProps) int {
		if c := b.PublishedAt.Compare(a.PublishedAt); c != 0 {
			return c
		}
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		switch {
		case b.ID > a.ID:
			return 1
		case b.ID < a.ID:
			return -1
		default:
			return 0
		}
	})

	return trimVideoList(limit, 3, props), nil
}

func GetVideoByID(ctx context.Context, db interface {
	database.VideosClient
	database.ChannelsClient
}, id string) (types.VideoProps, error) {
	vid, err := db.GetVideoByID(ctx, id, database.Video.WithChannel())
	if err != nil && !database.IsErrNotFound(err) {
		return types.VideoProps{}, errors.Wrap(err, "failed to get video")
	}
	if vid != nil && vid.R.Channel != nil {
		return types.VideoModelToProps(vid, types.ChannelModelToProps(vid.R.Channel)), nil
	}

	details, err := youtube.DefaultClient.GetVideoDetailsByID(id)
	if err != nil {
		return types.VideoProps{}, errors.Wrap(err, "failed to get video details")
	}
	if details.ChannelID == "" {
		return types.VideoProps{}, errors.New("failed to get video details, channel id is blank")
	}
	details.Title = resolveVideoTitle(details.Title, "", details.ID, details.Type)

	channel, _, err := CacheChannel(ctx, db, details.ChannelID)
	if err != nil {
		return types.VideoProps{}, errors.Wrap(err, "GetVideoByID.CacheChannel failed to cache channel")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
		defer cancel()
		_ = UpdateVideoCache(ctx, db, details)
	}()

	props := types.VideoProps{Video: details.Video, Channel: types.ChannelModelToProps(channel)}
	return props, nil

}

func UpdateVideoCache(ctx context.Context, db database.VideosClient, video *youtube.VideoDetails) error {
	existingTitle := ""
	if strings.TrimSpace(video.Title) == "" {
		existing, err := db.GetVideoByID(ctx, video.ID)
		if err != nil && !database.IsErrNotFound(err) {
			return err
		}
		if existing != nil {
			existingTitle = existing.Title
		}
	}
	title := resolveVideoTitle(video.Title, existingTitle, video.ID, video.Type)

	err := db.UpsertVideos(ctx, &models.Video{
		ID:          video.ID,
		ChannelID:   video.ChannelID,
		Type:        string(video.Type),
		PublishedAt: video.PublishedAt,
		Title:       title,
		Description: video.Description,
		Duration:    int64(video.Duration),
		Private:     video.Type == youtube.VideoTypePrivate,
	})
	return err
}

func EnsureVideoCached(ctx context.Context, db interface {
	database.VideosClient
	database.ChannelsClient
}, videoID string) error {
	_, err := db.GetVideoByID(ctx, videoID)
	if err == nil {
		return nil
	}
	if !database.IsErrNotFound(err) {
		return errors.Wrap(err, "failed to check cached video")
	}

	details, err := youtube.DefaultClient.GetVideoDetailsByID(videoID)
	if err != nil {
		return errors.Wrap(err, "failed to fetch video details")
	}
	if details.ChannelID == "" {
		return errors.New("video details missing channel id")
	}

	_, _, err = CacheChannel(ctx, db, details.ChannelID)
	if err != nil {
		return errors.Wrap(err, "failed to cache video channel")
	}

	if err := UpdateVideoCache(ctx, db, details); err != nil {
		return errors.Wrap(err, "failed to cache video")
	}

	return nil
}

type GetPlayerOptions struct {
	WithProgress bool
	WithSegments bool
}

func GetPlayerPropsWithOpts(ctx context.Context, db database.Client, userId, videoId string, opts ...GetPlayerOptions) (types.VideoPlayerProps, error) {
	var options GetPlayerOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	video, err := GetVideoByID(ctx, db, videoId)
	if err != nil {
		return types.VideoPlayerProps{}, errors.Wrap(err, "GetPlayerPropsWithOpts.GetVideoByID failed to get video")
	}

	playerProps := types.VideoPlayerProps{
		Authenticated: userId != "",
		Video:         video,
	}

	if options.WithProgress {
		views, err := GetUserViews(ctx, db, userId, videoId)
		if err != nil && !database.IsErrNotFound(err) {
			return types.VideoPlayerProps{}, errors.Wrap(err, "GetPlayerPropsWithOpts.database.DefaultClient.GetUserVideoView failed to get user video view")
		}

		for _, view := range views {
			if view.VideoID == videoId {
				playerProps.Video.Hidden = view.Hidden.Bool
				playerProps.Video.Progress = int(view.Progress)
				break
			}
		}
	}

	if options.WithSegments {
		if sponsorblock.C == nil {
			return playerProps, nil
		}
		segments, err := sponsorblock.C.GetVideoSegments(videoId)
		if err == nil {
			err := playerProps.AddSegments(segments...)
			if err != nil {
				return playerProps, errors.Wrap(err, "GetPlayerPropsWithOpts.playerProps.AddSegments failed to add segments")
			}
			return playerProps, nil
		}
		log.Warnf("failed to get sponsorblock segments for video %v: %v", videoId, err)
		return playerProps, nil
	}

	return playerProps, nil
}

func UpdateView(ctx context.Context, db database.ViewsClient, userId, videoId string, progress int, hidden bool) (int, error) {
	currentProgress, currentHidden, err := getCurrentViewState(ctx, db, userId, videoId)
	if err != nil {
		return 0, err
	}

	return upsertViewProgress(ctx, db, userId, videoId, currentProgress, currentHidden, progress, hidden)
}

func UpdateViewProgress(ctx context.Context, db database.ViewsClient, userId, videoId string, progress int) (int, error) {
	currentProgress, currentHidden, err := getCurrentViewState(ctx, db, userId, videoId)
	if err != nil {
		return 0, err
	}

	// Preserve hidden state when syncing progress from external sources like TV playback.
	return upsertViewProgress(ctx, db, userId, videoId, currentProgress, currentHidden, progress, currentHidden)
}

func getCurrentViewState(ctx context.Context, db database.ViewsClient, userId, videoId string) (int, bool, error) {
	views, err := db.GetUserViews(ctx, userId, videoId)
	if err != nil && !database.IsErrNotFound(err) {
		return 0, false, err
	}

	currentProgress := 0
	currentHidden := false
	for _, view := range views {
		if view.VideoID == videoId {
			currentProgress = int(view.Progress)
			currentHidden = view.Hidden.Bool
			break
		}
	}
	return currentProgress, currentHidden, nil
}

func upsertViewProgress(ctx context.Context, db database.ViewsClient, userId, videoId string, currentProgress int, currentHidden bool, progress int, hidden bool) (int, error) {
	progress = max(0, progress)
	if currentProgress == progress && currentHidden == hidden {
		return progress, nil
	}

	err := db.UpsertView(ctx, &models.View{
		VideoID:  videoId,
		UserID:   userId,
		Progress: int64(progress),
		Hidden:   null.BoolFrom(hidden),
	})
	if err != nil {
		return 0, err
	}

	return progress, nil
}

func GetUserViews(ctx context.Context, db database.ViewsClient, userId string, videos ...string) (map[string]*models.View, error) {
	views, err := db.GetUserViews(ctx, userId, videos...)
	if err != nil {
		return nil, errors.Wrap(err, "GetCompleteUserProgress.database.DefaultClient.GetAllUserViews failed to get user views")
	}

	viewsMap := make(map[string]*models.View)
	for _, v := range views {
		viewsMap[v.VideoID] = v
	}
	return viewsMap, nil
}

func GetUserVideoProgress(ctx context.Context, db database.ViewsClient, userId string, videos ...string) (map[string]int, error) {
	views, err := GetUserViews(ctx, db, userId, videos...)
	if err != nil {
		return nil, err
	}

	progress := make(map[string]int)
	for _, v := range views {
		progress[v.VideoID] = int(v.Progress)
	}

	return progress, nil
}

func VideoIDFromURL(link string) (string, bool) {
	var id string
	parsed, _ := url.Parse(link)

	switch {
	case parsed.Query().Get("v") != "":
		id = parsed.Query().Get("v")
	case parsed.Path != "":
		path := strings.Split(parsed.Path, "/")
		id = path[len(path)-1]
		if id == "" {
			return "", false
		}
	default:
		return "", false
	}
	id = strings.Trim(id, " ")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{5,}$`, id); matched {
		return id, true
	}
	return "", false
}

func trimVideoList(limit, batchSize int, videos []types.VideoProps) []types.VideoProps {
	if len(videos) > limit {
		return videos[:limit]
	}
	if len(videos) > batchSize {
		cutoff := len(videos) - (len(videos) % batchSize)
		return videos[:cutoff]
	}
	return videos
}

/*
Returns a list of video props for a channel filtered by type
*/
func GetChannelVideosFiltered(ctx context.Context, db database.VideosClient, limit int, filter types.VideoFilter, channelID string) ([]types.VideoProps, error) {
	opts := []database.VideoQuery{
		database.Video.Channel(channelID),
		database.Video.Limit(limit),
		database.Video.WithChannel(),
	}

	switch filter {
	case types.VideoFilterVideos:
		opts = append(opts, database.Video.TypeEq("video"))
	case types.VideoFilterStreams:
		opts = append(opts, database.Video.TypeEq("live_stream", "upcoming_stream", "stream_recording"))
	default:
		opts = append(opts, database.Video.TypeNot("private", "short"))
	}

	videos, err := db.FindVideos(ctx, opts...)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "GetChannelVideosFiltered.db.FindVideos failed to get videos")
	}

	var props []types.VideoProps
	for _, video := range videos {
		c := types.ChannelModelToProps(video.R.Channel)
		props = append(props, types.VideoModelToProps(video, c))
	}

	slices.SortFunc(props, func(a, b types.VideoProps) int {
		return b.PublishedAt.Compare(a.PublishedAt)
	})

	return props, nil
}

/*
Filters video props by type (used for filtering freshly cached videos)
*/
func filterVideosByType(videos []types.VideoProps, filter types.VideoFilter) []types.VideoProps {
	if filter == types.VideoFilterAll {
		return videos
	}

	var filtered []types.VideoProps
	for _, v := range videos {
		switch filter {
		case types.VideoFilterVideos:
			if v.Type == youtube.VideoTypeVideo {
				filtered = append(filtered, v)
			}
		case types.VideoFilterStreams:
			if v.Type == youtube.VideoTypeLiveStream || v.Type == youtube.VideoTypeUpcomingStream || v.Type == youtube.VideoTypeStreamRecording {
				filtered = append(filtered, v)
			}
		default:
			if v.Type != youtube.VideoTypePrivate && v.Type != youtube.VideoTypeShort {
				filtered = append(filtered, v)
			}
		}
	}
	return filtered
}
