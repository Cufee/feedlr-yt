package logic

import (
	"errors"
	"log"

	"github.com/byvko-dev/youtube-app/internal/api/youtube"
	"github.com/byvko-dev/youtube-app/internal/database"
	"github.com/byvko-dev/youtube-app/prisma/db"
	"github.com/ssoroka/slice"
)

/*
Saves last 3 videos for each channel to the database
*/
func CacheChannelVideos(channelIds ...string) error {
	for _, c := range channelIds {
		newVideos, err := youtube.C.GetChannelVideos(c, 3)
		if err != nil {
			return err
		}

		existingVideos, err := database.C.GetVideosByChannelID(0, c)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			return err
		}
		var existingIDs []string
		for _, v := range existingVideos {
			existingIDs = append(existingIDs, v.ID)
		}

		var models []database.VideoCreateModel
		for _, video := range newVideos {
			if slice.Contains(existingIDs, video.ID) {
				continue
			}
			models = append(models, database.VideoCreateModel{
				ID:          video.ID,
				URL:         video.URL,
				Title:       video.Title,
				Description: video.Description,
				Thumbnail:   video.Thumbnail,
			})
		}
		_, err = database.C.NewVideo(c, models...)
		if err != nil {
			log.Printf("Error saving videos for channel %s: %v", c, err)
			return err
		}
	}
	return nil
}

/*
Saves the channel to the database if it doesn't exist already and returns the channel model
*/
func CacheChannel(channelId string) (*db.ChannelModel, error) {
	exists, err := database.C.GetChannel(channelId)
	if err == nil {
		return exists, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}

	channel, err := youtube.C.GetChannel(channelId)
	if err != nil {
		return nil, err
	}

	cached, err := database.C.NewChannel(database.ChannelCreateModel{
		ID:          channel.ID,
		URL:         channel.URL,
		Title:       channel.Title,
		Description: channel.Description,
		Thumbnail:   channel.Thumbnail,
	})
	if err != nil {
		return nil, err
	}

	go CacheChannelVideos(channelId)

	return cached, nil
}
