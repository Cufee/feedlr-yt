package logic

import (
	"errors"
	"log"
	"time"

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
		newVideos, err := youtube.DefaultClient.GetChannelVideos(c, 3)
		if err != nil {
			return errors.Join(errors.New("CacheChannelVideos.youtube.C.GetChannelVideos"), err)
		}

		existingVideos, err := database.DefaultClient.GetVideosByChannelID(0, c)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.GetVideosByChannelID"), err)
		}
		var existingIDs []string
		for _, v := range existingVideos {
			existingIDs = append(existingIDs, v.ExternalID)

		}
		for _, video := range newVideos {
			if slice.Contains(existingIDs, video.ID) {
				continue
			}
			publishedAt, err := time.Parse(time.RFC3339, video.PublishedAt)
			if err != nil {
				log.Printf("Error parsing publishedAt %v", err)
			}
			models = append(models, database.VideoCreateModel{
				ChannelID:   c,
				Type:        string(video.Type),
				ID:          video.ID,
				URL:         video.URL,
				Title:       video.Title,
				Duration:    video.Duration,
				Thumbnail:   video.Thumbnail,
				Description: video.Description,
				PublishedAt: publishedAt,
			})
		}
	}

	if len(models) == 0 {
		return nil
	}
	err := database.DefaultClient.InsertChannelVideos(models...)
	if err != nil {
		return errors.Join(errors.New("CacheChannelVideos.database.DefaultClient.InsertChannelVideos"), err)
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
		return nil, errors.Join(errors.New("CacheChannel.database.DefaultClient.GetChannel"), err)
	}

	channel, err := youtube.DefaultClient.GetChannel(channelId)
	if err != nil {
		return nil, errors.Join(errors.New("CacheChannel.youtube.C.GetChannel"), err)
	}

	cached, err := database.DefaultClient.NewChannel(database.ChannelCreateModel{
		ID:          channel.ID,
		URL:         channel.URL,
		Title:       channel.Title,
		Description: channel.Description,
		Thumbnail:   channel.Thumbnail,
	})
	if err != nil {
		return nil, errors.Join(errors.New("CacheChannel.database.DefaultClient.NewChannel"), err)
	}

	return cached, nil
}
