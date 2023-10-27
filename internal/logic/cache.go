package logic

import (
	"errors"
	"log"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/ssoroka/slice"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
Saves last 3 videos for each channel to the database
*/
func CacheChannelVideos(channelIds ...string) error {
	var models []database.VideoCreateModel

	for _, c := range channelIds {
		newVideos, err := youtube.C.GetChannelVideos(c, 3)
		if err != nil {
			return err
		}

		existingVideos, err := database.DefaultClient.GetVideosByChannelID(0, c)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		var existingIDs []string
		for _, v := range existingVideos {
			existingIDs = append(existingIDs, v.ID)

		}
		for _, video := range newVideos {
			if slice.Contains(existingIDs, video.ID) {
				continue
			}
			models = append(models, database.VideoCreateModel{
				ChannelID:   c,
				ID:          video.ID,
				URL:         video.URL,
				Title:       video.Title,
				Duration:    video.Duration,
				Description: video.Description,
				Thumbnail:   video.Thumbnail,
			})
		}
	}

	if len(models) == 0 {
		log.Printf("No new videos to save")
		return nil
	}
	err := database.DefaultClient.InsertChannelVideos(models...)
	if err != nil {
		log.Printf("Error saving videos %v", err)
		return err
	}
	return nil
}

/*
Saves the channel to the database if it doesn't exist already and returns the channel model
*/
func CacheChannel(channelId string) (*models.Channel, error) {
	exists, err := database.DefaultClient.GetChannel(channelId)
	if err == nil {
		return exists, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	channel, err := youtube.C.GetChannel(channelId)
	if err != nil {
		return nil, err
	}

	cached, err := database.DefaultClient.NewChannel(database.ChannelCreateModel{
		ID:          channel.ID,
		URL:         channel.URL,
		Title:       channel.Title,
		Description: channel.Description,
		Thumbnail:   channel.Thumbnail,
	})
	if err != nil {
		return nil, err
	}

	return cached, nil
}
