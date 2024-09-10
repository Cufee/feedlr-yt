package logic

import (
	"context"

	"net/url"
	"regexp"
	"slices"
	"time"

	"strings"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/friendsofgo/errors"
	"github.com/gofiber/fiber/v2/log"
	"github.com/volatiletech/null/v8"
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

	allVideos, err := GetChannelVideos(ctx, db, 24, channelIds...)
	if err != nil {
		return nil, errors.Wrap(err, "GetUserSubscriptionsProps.GetChannelVideos failed to get channel videos")
	}

	videoIds := make([]string, len(allVideos))
	for i, v := range allVideos {
		videoIds[i] = v.ID
	}

	views, err := GetUserViews(ctx, db, userId, videoIds...)
	if err != nil {
		return nil, errors.Wrap(err, "GetUserSubscriptionsProps.GetCompleteUserProgress failed to get user progress")
	}

	var feed types.UserVideoFeedProps
	for _, video := range allVideos {
		if v, ok := views[video.ID]; ok {
			if v.Hidden.Bool {
				continue
			}
			video.Progress = int(v.Progress)
		}
		feed.Videos = append(feed.Videos, video)
	}

	return &feed, nil
}

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetRecentVideosProps(ctx context.Context, db interface {
	database.VideosClient
	database.ViewsClient
}, userId string) (*types.UserVideoFeedProps, error) {
	views, err := db.GetRecentUserViews(ctx, userId, 24)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, errors.Wrap(err, "GetCompleteUserProgress.database.DefaultClient.GetAllUserViews failed to get user views")
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

	var feed types.UserVideoFeedProps
	for _, video := range videos {
		v := types.VideoModelToProps(video, types.ChannelModelToProps(video.R.Channel))
		if view, ok := progress[video.ID]; ok {
			v.Progress = int(view.Progress)
		}
		feed.Videos = append(feed.Videos, v)
	}
	slices.SortFunc(feed.Videos, func(a, b types.VideoProps) int {
		var au, bu time.Time
		if v, ok := progress[a.ID]; ok {
			au = v.UpdatedAt
		}
		if v, ok := progress[b.ID]; ok {
			bu = v.UpdatedAt
		}
		return bu.Compare(au)
	})

	return &feed, nil
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
		return b.PublishedAt.Compare(a.PublishedAt)
	})

	return trimVideoList(limit, 12, props), nil
}

func GetVideoByID(ctx context.Context, db interface {
	database.VideosClient
	database.ChannelsClient
}, id string) (types.VideoProps, error) {
	vid, err := db.GetVideoByID(ctx, id, database.Video.WithChannel())
	if err != nil && !database.IsErrNotFound(err) {
		return types.VideoProps{}, errors.Wrap(err, "GetVideoByID.database.DefaultClient.GetVideoByID failed to get video")
	}
	if vid != nil && vid.R.Channel != nil {
		return types.VideoModelToProps(vid, types.ChannelModelToProps(vid.R.Channel)), nil
	}

	details, err := youtube.DefaultClient.GetVideoDetailsByID(id)
	if err != nil {
		return types.VideoProps{}, errors.Wrap(err, "GetVideoByID.youtube.DefaultClient.GetVideoPlayerDetails failed to get video details")
	}

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
	published, _ := time.Parse(time.RFC3339, video.PublishedAt)
	err := db.UpsertVideos(ctx, &models.Video{
		ID:          video.ID,
		ChannelID:   video.ChannelID,
		Type:        string(video.Type),
		PublishedAt: published,
		Title:       video.Title,
		Description: video.Description,
		Duration:    int64(video.Duration),
		Private:     video.Type == youtube.VideoTypePrivate,
	})
	return err
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

func UpdateView(ctx context.Context, db database.ViewsClient, userId, videoId string, progress int, hidden bool) error {
	err := db.UpsertView(ctx, &models.View{
		VideoID:  videoId,
		UserID:   userId,
		Progress: int64(progress),
		Hidden:   null.BoolFrom(hidden),
	})
	return err
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
