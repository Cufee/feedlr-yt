package database

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *Client) GetVideoByID(id string) (*models.Video, error) {
	video := &models.Video{}
	ctx, cancel := c.Ctx()
	defer cancel()

	err := c.Collection(models.VideoCollection).FindOne(ctx, bson.M{"eid": id}).Decode(video)
	if err != nil {
		return nil, err
	}
	return video, nil
}

func (c *Client) GetVideosByChannelID(limit int, channelIds ...string) ([]models.Video, error) {
	videos := []models.Video{}
	ctx, cancel := c.Ctx()
	defer cancel()

	opts := options.Find().SetSort(bson.M{"publishedAt": -1})
	cur, err := c.Collection(models.VideoCollection).Find(ctx, bson.M{"channelId": bson.M{"$in": channelIds}}, opts)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &videos)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (c *Client) GetLatestVideos(limit int, page int, channelIds ...string) ([]models.Video, error) {
	videos := []models.Video{}
	ctx, cancel := c.Ctx()
	defer cancel()

	filter := bson.M{}
	if len(channelIds) > 0 {
		filter = bson.M{"channelId": bson.M{"$in": channelIds}}
	}

	opts := options.Find().SetSort(bson.M{"publishedAt": -1})
	opts.SetSkip(int64(page * limit))
	opts.SetLimit(int64(limit))
	cur, err := c.Collection(models.VideoCollection).Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &videos)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (c *Client) GetLatestChannelVideos(id string, limit int) ([]models.Video, error) {
	videos := []models.Video{}
	opts := options.Find()
	opts.SetSort(bson.M{"publishedAt": -1})
	opts.SetLimit(int64(limit))
	ctx, cancel := c.Ctx()
	defer cancel()

	cur, err := c.Collection(models.VideoCollection).Find(ctx, bson.M{"channelId": id}, opts)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &videos)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

type VideoCreateModel struct {
	ID          string
	URL         string
	Title       string
	Duration    int
	ChannelID   string
	PublishedAt time.Time
	Description string
	Thumbnail   string
}

func (c *Client) InsertChannelVideos(videos ...VideoCreateModel) error {
	payload := []*models.Video{}
	for _, video := range videos {
		v := models.NewVideo(video.ID, video.URL, video.Title, video.ChannelID, video.PublishedAt, models.VideoOptions{Thumbnail: &video.Thumbnail, Duration: &video.Duration, Description: &video.Description})
		v.Prepare()
		payload = append(payload, v)
	}

	var writes []mongo.WriteModel
	for _, video := range payload {
		writes = append(writes, mongo.NewInsertOneModel().SetDocument(*video))
	}

	ctx, cancel := c.Ctx()
	defer cancel()
	_, err := c.Collection(models.VideoCollection).BulkWrite(ctx, writes)
	return err
}

func (c *Client) GetUserVideoView(user primitive.ObjectID, video string) (*models.VideoView, error) {
	view := &models.VideoView{}
	ctx, cancel := c.Ctx()
	defer cancel()

	err := c.Collection(models.VideoViewCollection).FindOne(ctx, bson.M{"userId": user, "videoId": video}).Decode(view)
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (c *Client) GetAllUserViews(user primitive.ObjectID) ([]models.VideoView, error) {
	views := []models.VideoView{}
	ctx, cancel := c.Ctx()
	defer cancel()

	cur, err := c.Collection(models.VideoViewCollection).Find(ctx, bson.M{"userId": user})
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &views)
	if err != nil {
		return nil, err
	}
	return views, nil
}

func (c *Client) UpsertView(user primitive.ObjectID, video string, progress int) (*models.VideoView, error) {
	view := models.NewVideoView(user, video, models.VideoViewOptions{Progress: &progress})
	view.Progress = progress
	view.Model.UpdatedAt = time.Now()
	view.Prepare()
	opts := options.Update().SetUpsert(true)

	ctx, cancel := c.Ctx()
	defer cancel()

	res, err := c.Collection(models.VideoViewCollection).UpdateOne(ctx, bson.M{"userId": user, "videoId": video}, bson.M{"$set": view}, opts)
	if err != nil {
		return nil, err
	}
	if res.UpsertedID != nil {
		return view, view.ParseID(res.UpsertedID)
	}
	return view, nil
}
