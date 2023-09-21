package api

import "github.com/byvko-dev/youtube-app/internal/types"

func GetUserChannels(uid string) ([]types.Channel, error) {
	var channels []types.Channel = []types.Channel{{
		ID:        "CH11111",
		Title:     "Android Police",
		Thumbnail: "",
		Favorite:  true,
		Videos: []types.Video{{
			ID:          "VID111111",
			ChannelID:   "CH11111",
			Title:       "Video 1",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 1",
		}, {
			ID:          "VID222222",
			ChannelID:   "CH11111",
			Title:       "Video 2",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 2",
		}, {
			ID:          "VID333333",
			ChannelID:   "CH11111",
			Title:       "Video 3",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 3",
		}},
	}, {
		ID:        "CH222222",
		Title:     "Some Name",
		Thumbnail: "",
		Videos: []types.Video{{
			ID:          "VID444444",
			ChannelID:   "CH222222",
			Title:       "Video 1 Video 1 Video 1 Video 1 Video 1 Video 1 Video 1",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 1",
		}},
	}, {
		ID:        "CH333333",
		Title:     "Some Name",
		Thumbnail: "",
		Videos: []types.Video{{
			ID:          "VID555555",
			ChannelID:   "CH333333",
			Title:       "Video 1 Video 1 Video 1 Video 1 Video 1 Video 1 Video 1",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 1",
		}},
	}, {
		ID:        "CH444444",
		Title:     "Some Name",
		Thumbnail: "",
		Videos: []types.Video{{
			ID:          "VID666666",
			ChannelID:   "CH444444",
			Title:       "Video 1 Video 1 Video 1 Video 1 Video 1 Video 1 Video 1",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 1",
		}},
	}, {
		ID:        "CH555555",
		Title:     "Some Name",
		Thumbnail: "",
		Videos: []types.Video{{
			ID:          "VID777777",
			ChannelID:   "CH555555",
			Title:       "Video 1 Video 1 Video 1 Video 1 Video 1 Video 1 Video 1",
			Thumbnail:   "https://vid.puffyan.us/vi/_dQPZaQVzsw/maxres.jpg",
			Description: "Description 1",
		}},
	}}

	for _, c := range channels {
		for i := range c.Videos {
			c.Videos[i].URL = c.Videos[i].BuildURL()
		}
	}

	return channels, nil
}
