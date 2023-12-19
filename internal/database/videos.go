package database

import (
	"time"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type GetVideoOptions struct {
	WithChannel bool
}

func (c *Client) GetVideoByID(id string, opts ...GetVideoOptions) (*models.Video, error) {
	ctx, cancel := c.Ctx()
	defer cancel()

	var options GetVideoOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	if !options.WithChannel {
		video := &models.Video{}
		err := c.Collection(video.CollectionName()).FindOne(ctx, bson.M{"eid": id}).Decode(video)
		if err != nil {
			return nil, err
		}
		return video, nil
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"eid": id}})
	stages = append(stages, bson.M{"$limit": 1})
	if options.WithChannel {
		lookup := bson.M{
			"from":         models.ChannelCollection,
			"localField":   "channelId",
			"foreignField": "eid",
			"as":           "channel",
		}
		stages = append(stages, bson.M{"$lookup": lookup})
		stages = append(stages, bson.M{"$addFields": bson.M{"channel": bson.M{"$arrayElemAt": bson.A{"$channel", 0}}}})
	}

	var videos []models.Video
	cur, err := c.Collection(models.VideoCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &videos)
	if err != nil {
		return nil, err
	}
	if len(videos) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return &videos[0], nil
}

func (c *Client) GetVideosByChannelID(limit int, channelIds ...string) ([]models.Video, error) {
	videos := []models.Video{}
	ctx, cancel := c.Ctx()
	defer cancel()

	opts := options.Find().SetSort(bson.M{"publishedAt": -1})
	cur, err := c.Collection(models.VideoCollection).Find(ctx, bson.M{"channelId": bson.M{"$in": channelIds}, "type": bson.M{"$ne": "private"}}, opts)
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
	Type        string
	ID          string
	URL         string
	Title       string
	Duration    int
	ChannelID   string
	PublishedAt time.Time
	Description string
	Thumbnail   string
}

func (c *Client) UpdateVideos(upsert bool, videos ...VideoCreateModel) error {
	payload := []*models.Video{}
	for _, video := range videos {
		v := models.NewVideo(video.ID, video.Type, video.URL, video.Title, video.ChannelID, video.PublishedAt, models.VideoOptions{Thumbnail: &video.Thumbnail, Duration: &video.Duration, Description: &video.Description})
		v.Prepare()
		payload = append(payload, v)
	}

	var writes []mongo.WriteModel
	for _, video := range payload {
		model := mongo.NewUpdateOneModel()
		model.SetUpsert(upsert)
		model.SetUpdate(bson.M{"$set": video})
		model.SetFilter(bson.M{"eid": video.ExternalID})
		writes = append(writes, model)
	}

	ctx, cancel := c.Ctx()
	defer cancel()
	_, err := c.Collection(models.VideoCollection).BulkWrite(ctx, writes)
	return err
}

func (c *Client) InsertChannelVideos(videos ...VideoCreateModel) error {
	payload := []*models.Video{}
	for _, video := range videos {
		v := models.NewVideo(video.ID, video.Type, video.URL, video.Title, video.ChannelID, video.PublishedAt, models.VideoOptions{Thumbnail: &video.Thumbnail, Duration: &video.Duration, Description: &video.Description})
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

func (c *Client) GetUserViews(user primitive.ObjectID, videos ...string) ([]models.VideoView, error) {
	views := []models.VideoView{}
	ctx, cancel := c.Ctx()
	defer cancel()

	cur, err := c.Collection(models.VideoViewCollection).Find(ctx, bson.M{"userId": user, "videoId": bson.M{"$in": videos}})
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
