package logic

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube/client"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v2/log"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	videos, err := database.DefaultClient.GetVideosByChannelID(0, channelIds...)
	if err != nil {
		return nil, err
	}

	var props []types.VideoProps
	for _, vid := range videos {
		v := types.VideoProps{
			Video: client.Video{
				ID:          vid.ID,
				URL:         vid.URL,
				Title:       vid.Title,
				Duration:    vid.Duration,
				Thumbnail:   vid.Thumbnail,
				Description: vid.Description,
			},
			ChannelID: vid.ChannelId,
		}
		props = append(props, v)
	}

	return props, nil
}

func GetVideoByID(id string) (types.VideoProps, error) {
	vid, err := database.DefaultClient.GetVideoByID(id)
	if err != nil {
		return types.VideoProps{}, err
	}

	v := types.VideoProps{
		Video: client.Video{
			ID:          vid.ID,
			URL:         vid.URL,
			Title:       vid.Title,
			Duration:    vid.Duration,
			Thumbnail:   vid.Thumbnail,
			Description: vid.Description,
		},
		ChannelID: vid.ChannelId,
	}

	return v, nil
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
		return types.VideoPlayerProps{}, err
	}

	playerProps := types.VideoPlayerProps{
		Video: video,
	}

	if options.WithProgress {
		progress, err := database.DefaultClient.GetUserVideoView(userId, videoId)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return types.VideoPlayerProps{}, err
		}
		if progress != nil {
			playerProps.Video.Progress = progress.Progress
		}
	}

	if options.WithSegments {
		segments, err := sponsorblock.C.GetVideoSegments(videoId)
		if err == nil {
			err = playerProps.AddSegments(segments...)
			return playerProps, err
		}
		log.Warnf("failed to get sponsorblock segments for video %v: %v", videoId, err)
		return playerProps, nil
	}

	return playerProps, nil
}

func UpdateViewProgress(userId, videoId string, progress int) error {
	_, err := database.DefaultClient.UpsertView(userId, videoId, progress)
	return err
}

func GetCompleteUserProgress(userId string) (map[string]int, error) {
	views, err := database.DefaultClient.GetAllUserViews(userId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return make(map[string]int), nil
		}
		return nil, err
	}

	progress := make(map[string]int)
	for _, v := range views {
		progress[v.VideoId] = v.Progress
	}

	return progress, nil
}
