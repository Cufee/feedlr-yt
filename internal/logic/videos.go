package logic

import (
	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/internal/types"
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
