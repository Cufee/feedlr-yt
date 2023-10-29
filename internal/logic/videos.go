package logic

import (
	"errors"
	"net/url"
	"regexp"

	"strings"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v2/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/slices"
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

	progress, err := GetCompleteUserProgress(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetCompleteUserProgress failed to get user progress"), err)
	}

	// Get videos for each channel and add them to the props
	channelsMap := make(map[string]types.ChannelProps)
	var channelIds []string
	for _, c := range channels {
		channelsMap[c.ID] = c
		channelIds = append(channelIds, c.ID)
	}

	limit := 48                                                // 48 can be divided by 1, 2, 3, 4
	allVideos, err := GetLatestVideos(limit, 0, channelIds...) // TODO: pagination
	if err != nil {
		return nil, errors.Join(errors.New("GetUserSubscriptionsProps.GetChannelVideos failed to get channel videos"), err)
	}

	cutoff := limit
	if len(allVideos) > 12 {
		cutoff = len(allVideos) - (len(allVideos) % 12)
	}

	var feed types.UserVideoFeedProps
	for i, video := range allVideos {
		if i >= cutoff {
			break
		}
		video.Progress = progress[video.ID]
		if video.Progress == 0 {
			feed.NewVideos = append(feed.NewVideos, video)
		}
		feed.Videos = append(feed.Videos, types.VideoWithChannelProps{
			VideoProps:       video,
			ChannelID:        video.ChannelID,
			ChannelTitle:     channelsMap[video.ChannelID].Title,
			ChannelThumbnail: channelsMap[video.ChannelID].Thumbnail,
		})
	}
	slices.Reverse(feed.NewVideos)
	return &feed, nil
}

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	videos, err := database.DefaultClient.GetVideosByChannelID(0, channelIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetChannelVideos.database.DefaultClient.GetVideosByChannelID failed to get videos"), err)
	}

	var props []types.VideoProps
	for _, vid := range videos {
		props = append(props, types.VideoModelToProps(&vid))
	}

	return props, nil
}

/*
Returns a chronological list of video props for provided channels
*/
func GetLatestVideos(limit int, page int, channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	videos, err := database.DefaultClient.GetLatestVideos(limit, page, channelIds...)
	if err != nil {
		return nil, errors.Join(errors.New("GetLatestVideos.database.DefaultClient.GetLatestVideos failed to get videos"), err)
	}

	var props []types.VideoProps
	for _, vid := range videos {
		props = append(props, types.VideoModelToProps(&vid))
	}

	return props, nil
}

func GetVideoByID(id string) (types.VideoProps, error) {
	vid, err := database.DefaultClient.GetVideoByID(id)
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

	return types.VideoModelToProps(vid), nil
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
		oid, err := primitive.ObjectIDFromHex(userId)
		if err != nil {
			return types.VideoPlayerProps{}, errors.Join(errors.New("GetPlayerPropsWithOpts.primitive.ObjectIDFromHex failed to parse userId"), err)
		}
		progress, err := database.DefaultClient.GetUserVideoView(oid, videoId)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return types.VideoPlayerProps{}, errors.Join(errors.New("GetPlayerPropsWithOpts.database.DefaultClient.GetUserVideoView failed to get user video view"), err)
		}
		if progress != nil {
			playerProps.Video.Progress = progress.Progress
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

func GetCompleteUserProgress(userId string) (map[string]int, error) {
	oid, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.Join(errors.New("GetCompleteUserProgress.primitive.ObjectIDFromHex failed to parse userId"), err)
	}

	views, err := database.DefaultClient.GetAllUserViews(oid)
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
