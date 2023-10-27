package database

import (
	"errors"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *Client) GetVideoByID(id string) (*models.Video, error) {
	video := &models.Video{}
	return video, mgm.Coll(video).First(bson.M{"_id": id}, video)
}

func (c *Client) GetVideosByChannelID(limit int, channelIds ...string) ([]models.Video, error) {
	videos := []models.Video{}
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	return videos, mgm.Coll(&models.Video{}).SimpleFind(&videos, bson.M{"channelId": bson.M{"$in": channelIds}}, opts)
}

func (c *Client) GetLatestChannelVideos(id string, limit int) ([]models.Video, error) {
	videos := []models.Video{}
	opts := options.Find()
	opts.SetSort(bson.M{"createdAt": -1})
	opts.SetLimit(int64(limit))
	return videos, mgm.Coll(&models.Video{}).SimpleFind(&videos, bson.M{"channelId": id}, opts)
}

type VideoCreateModel struct {
	ID          string
	URL         string
	Title       string
	Duration    int
	Description string
	Thumbnail   string
}

func (c *Client) InsertChannelVideos(channel string, videos ...VideoCreateModel) error {
	payload := []*models.Video{}
	for _, video := range videos {
		payload = append(payload, models.NewVideo(video.ID, video.URL, video.Title, channel, models.VideoOptions{Thumbnail: &video.Thumbnail, Duration: &video.Duration, Description: &video.Description}))
	}

	var writes []mongo.WriteModel
	for _, video := range payload {
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(video))
	}

	_, err := mgm.Coll(&models.Video{}).BulkWrite(mgm.Ctx(), writes)
	return err
}

func (c *Client) GetUserVideoView(user, video string) (*models.VideoView, error) {
	view := &models.VideoView{}
	return view, mgm.Coll(view).First(bson.M{"userId": user, "videoId": video}, view)
}

func (c *Client) GetAllUserViews(user string) ([]models.VideoView, error) {
	virews := []models.VideoView{}
	return virews, mgm.Coll(&models.VideoView{}).SimpleFind(&virews, bson.M{"userId": user})
}

func (c *Client) UpsertView(user, video string, progress int) (*models.VideoView, error) {
	view := &models.VideoView{}
	err := mgm.Coll(view).First(bson.M{"userId": user, "videoId": video}, view)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			view = models.NewVideoView(user, video, models.VideoViewOptions{Progress: &progress})
			return view, mgm.Coll(view).Create(view)
		}
		return nil, err
	}
	view.Progress = progress
	_, err = mgm.Coll(view).UpdateByID(mgm.Ctx(), view.ID, bson.M{"$set": view})
	return view, err
}
