package logic

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"time"

	"strings"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v2/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
Returns a list of channel props with videos for all user subscriptions
*/
func GetUserVideosProps(userId string) (*types.UserVideoFeedProps, error) {
	// Get channels and convert them to WithVideo props
	channels, err := GetUserSubscribedChannels(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetUserSubscribedChannels failed to get user subscribed channels"), err)
	}

	// Get videos for each channel and add them to the props
	channelsMap := make(map[string]types.ChannelProps)
	var channelIds []string
	for _, c := range channels {
		channelsMap[c.ID] = c
		channelIds = append(channelIds, c.ID)
	}

	allVideos, err := GetChannelVideos(24, channelIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetChannelVideos failed to get channel videos"), err)
	}

	slices.SortFunc(allVideos, func(a, b types.VideoProps) int {
		aT, _ := time.Parse(time.RFC3339, a.PublishedAt)
		bT, _ := time.Parse(time.RFC3339, b.PublishedAt)
		return int(bT.Unix() - aT.Unix())
	})

	videos := trimVideoList(24, 12, allVideos) // 12 can be divided by 1, 2, 3, 4 for a nice grid
	videoIds := make([]string, len(videos))
	for i, v := range videos {
		videoIds[i] = v.ID
	}

	progress, err := GetUserVideoProgress(userId, videoIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetCompleteUserProgress failed to get user progress"), err)
	}

	var feed types.UserVideoFeedProps
	for _, video := range videos {
		video.Progress = progress[video.ID]
		feed.Videos = append(feed.Videos, video)
	}

	return &feed, nil
}

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(limit int, channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	channels, err := database.DefaultClient.GetChannelsByID(channelIds, database.ChannelGetOptions{WithVideos: true, VideosLimit: limit})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, errors.Join(errors.New("GetChannelVideos.database.DefaultClient.GetVideosByChannelID failed to get videos"), err)
	}

	var props []types.VideoProps
	for _, channel := range channels {
		c := types.ChannelModelToProps(&channel)
		for _, video := range channel.Videos {
			props = append(props, types.VideoModelToProps(&video, c))
		}
	}

	return trimVideoList(limit, 12, props), nil
}

func GetVideoByID(id string) (types.VideoProps, error) {
	vid, err := database.DefaultClient.GetVideoByID(id, database.GetVideoOptions{WithChannel: true})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			video, err := youtube.DefaultClient.GetVideoByID(id)
			if err != nil {
				return types.VideoProps{}, errors.Join(errors.New("GetVideoByID.youtube.DefaultClient.GetVideoPlayerDetails failed to get video details"), err)
			}
			return types.VideoProps{Video: *video}, nil
		}
		return types.VideoProps{}, errors.Join(errors.New("GetVideoByID.database.DefaultClient.GetVideoByID failed to get video"), err)
	}
	return types.VideoModelToProps(vid, types.ChannelModelToProps(vid.Channel)), nil
}

func UpdateVideoCache(videoId string) error {
	video, err := youtube.DefaultClient.GetVideoByID(videoId)
	if err != nil {
		return errors.Join(errors.New("UpdateVideoCache.youtube.DefaultClient.GetVideoPlayerDetails failed to get video details"), err)
	}

	err = database.DefaultClient.UpdateVideos(false, database.VideoCreateModel{
		Type:        string(video.Type),
		ID:          video.ID,
		URL:         video.URL,
		Title:       video.Title,
		Duration:    video.Duration,
		Thumbnail:   video.Thumbnail,
		Description: video.Description,
	})
	return err
}

type GetPlayerOptions struct {
	WithProgress bool
	WithSegments bool
}

func GetPlayerPropsWithOpts(userId, videoId string, opts ...GetPlayerOptions) (types.VideoPlayerProps, error) {
	var options GetPlayerOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	video, err := GetVideoByID(videoId)
	if err != nil {
		return types.VideoPlayerProps{}, errors.Join(errors.New("GetPlayerPropsWithOpts.GetVideoByID failed to get video"), err)
	}

	playerProps := types.VideoPlayerProps{
		Video: video,
	}

	if options.WithProgress {
		progress, err := GetUserVideoProgress(userId, videoId)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return types.VideoPlayerProps{}, errors.Join(errors.New("GetPlayerPropsWithOpts.database.DefaultClient.GetUserVideoView failed to get user video view"), err)
		}
		if progress != nil {
			playerProps.Video.Progress = progress[videoId]
		}
	}

	if options.WithSegments {
		segments, err := sponsorblock.C.GetVideoSegments(videoId)
		if err == nil {
			err := playerProps.AddSegments(segments...)
			if err != nil {
				return playerProps, errors.Join(errors.New("GetPlayerPropsWithOpts.playerProps.AddSegments failed to add segments"), err)
			}
			return playerProps, nil
		}
		log.Warnf("failed to get sponsorblock segments for video %v: %v", videoId, err)
		return playerProps, nil
	}

	return playerProps, nil
}

func UpdateViewProgress(userId, videoId string, progress int) error {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return err
	}
	_, err = database.DefaultClient.UpsertView(oid, videoId, progress)
	return err
}

func GetUserVideoProgress(userId string, videos ...string) (map[string]int, error) {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetCompleteUserProgress.primitive.ObjectIDFromHex failed to parse userId"), err)
	}

	views, err := database.DefaultClient.GetUserViews(oid, videos...)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return make(map[string]int), nil
		}
		return nil, errors.Join(errors.New("GetCompleteUserProgress.database.DefaultClient.GetAllUserViews failed to get user views"), err)
	}

	progress := make(map[string]int)
	for _, v := range views {
		progress[v.VideoId] = v.Progress
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
