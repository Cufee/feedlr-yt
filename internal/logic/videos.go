package logic

import (
	"errors"

	"github.com/byvko-dev/youtube-app/internal/api/sponsorblock"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
	"github.com/byvko-dev/youtube-app/prisma/db"
)

/*
Returns a list of video props for provided channels
*/
func GetChannelVideos(channelIds ...string) ([]types.VideoProps, error) {
	if len(channelIds) == 0 {
		return nil, nil
	}

	videos, err := database.C.GetVideosByChannelID(0, channelIds...)
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
				Description: vid.Description,
			},
			ChannelID: vid.ChannelID,
		}
		v.Thumbnail, _ = vid.Thumbnail()
		props = append(props, v)
	}

	return props, nil
}

func GetVideoByID(id string) (types.VideoProps, error) {
	vid, err := database.C.GetVideoByID(id)
	if err != nil {
		return types.VideoProps{}, err
	}

	v := types.VideoProps{
		Video: client.Video{
			ID:          vid.ID,
			URL:         vid.URL,
			Title:       vid.Title,
			Description: vid.Description,
		},
		ChannelID: vid.ChannelID,
	}
	v.Thumbnail, _ = vid.Thumbnail()

	return v, nil
}

type GetVideoOptions struct {
	WithProgress bool
	WithSegments bool
}

func GetVideoWithOptions(userId, videoId string, opts ...GetVideoOptions) (types.VideoProps, error) {
	var options GetVideoOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	video, err := GetVideoByID(videoId)
	if err != nil {
		return types.VideoProps{}, err
	}

	if options.WithProgress {
		progress, err := database.C.GetUserVideoView(userId, videoId)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return video, nil
			}
			return types.VideoProps{}, err
		}
		video.Progress = progress.Progress
	}

	if options.WithSegments {
		segments, err := sponsorblock.C.GetVideoSegments(videoId)
		if err != nil {
			return types.VideoProps{}, err
		}
		err = video.AddSegments(segments...)
		if err != nil {
			return types.VideoProps{}, err
		}
	}

	return video, nil
}

func UpdateViewProgress(userId, videoId string, progress int) error {
	_, err := database.C.UpsertView(userId, videoId, progress)
	return err
}

func GetCompleteUserProgress(userId string) (map[string]int, error) {
	views, err := database.C.GetAllUserViews(userId)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return make(map[string]int), nil
		}
		return nil, err
	}

	progress := make(map[string]int)
	for _, v := range views {
		progress[v.VideoID] = v.Progress
	}

	return progress, nil
}
