package logic

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/gofiber/fiber/v2/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		return nil, errors.Join(errors.New("GetChannelVideos.database.DefaultClient.GetVideosByChannelID failed to get videos"), err)
	}

	var props []types.VideoProps
	for _, vid := range videos {
		v := types.VideoProps{
			Video: youtube.Video{
				ID:          vid.ExternalID,
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
		if errors.Is(err, mongo.ErrNoDocuments) {
			video, err := youtube.DefaultClient.GetVideoByID(id)
			if err != nil {
				return types.VideoProps{}, errors.Join(errors.New("GetVideoByID.youtube.DefaultClient.GetVideoPlayerDetails failed to get video details"), err)
			}
			return types.VideoProps{Video: video.Video, ChannelID: video.ChannelID}, nil
		}
		return types.VideoProps{}, errors.Join(errors.New("GetVideoByID.database.DefaultClient.GetVideoByID failed to get video"), err)
	}

	v := types.VideoProps{
		Video: youtube.Video{
			ID:          vid.ExternalID,
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
