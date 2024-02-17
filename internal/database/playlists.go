package database

import (
	"github.com/cufee/feedlr-yt/internal/database/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (c *Client) NewPlaylist(userId primitive.ObjectID, name string, videoIds []string) (*models.Playlist, error) {
	ctx, cancel := c.Ctx()
	defer cancel()
	list := models.NewPlaylist(userId, name, videoIds)
	list.Prepare()
	res, err := c.Collection(list.CollectionName()).InsertOne(ctx, list)
	if err != nil {
		return nil, err
	}
	return list, list.ParseID(res.InsertedID)
}

type PlaylistGetOptions struct {
	WithVideos bool
	WithUser   bool
}

func (c *Client) AllUserPlaylists(userId primitive.ObjectID, o ...PlaylistGetOptions) ([]models.Playlist, error) {
	var opts PlaylistGetOptions
	if len(o) > 0 {
		opts = o[0]
	}

	playlists := []models.Playlist{}
	ctx, cancel := c.Ctx()
	defer cancel()

	if !opts.WithVideos && !opts.WithUser {
		findOpts := options.Find()
		findOpts.SetSort(bson.M{"createdAt": -1})
		cur, err := c.Collection(models.PlaylistCollection).Find(ctx, bson.M{"userId": userId}, findOpts)
		if err != nil {
			return nil, err
		}
		return playlists, cur.All(ctx, &playlists)
	}

	var stages []interface{}
	stages = append(stages, bson.M{"$match": bson.M{"userId": userId}})
	stages = append(stages, bson.M{"$sort": bson.M{"createdAt": -1}})
	if opts.WithVideos {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.VideoCollection,
				"localField":   "videoIds",
				"foreignField": "eid",
				"as":           "videos",
				"let":          bson.M{"videoId": "$videoIds"},
				"pipeline": mongo.Pipeline{
					{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$in", Value: bson.A{"$eid", "$$videoId"}}}}}}},
					{{Key: "$sort", Value: bson.D{{Key: "publishedAt", Value: -1}}}},
				},
			}})
	}
	if opts.WithUser {
		stages = append(stages, bson.M{
			"$lookup": bson.M{
				"from":         models.UserCollection,
				"localField":   "userId",
				"foreignField": "_id",
				"as":           "user",
			},
		})
	}

	cur, err := c.Collection(models.PlaylistCollection).Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	err = cur.All(ctx, &playlists)
	if err != nil {
		return nil, err
	}

	return playlists, nil
}

// func (c *Client) FindSubscription(userId primitive.ObjectID, channelId string, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
// 	var options SubscriptionGetOptions
// 	if len(opts) > 0 {
// 		options = opts[0]
// 	}

// 	subscription := &models.UserSubscription{}
// 	ctx, cancel := c.Ctx()
// 	defer cancel()

// 	if !options.WithChannel && !options.WithUser {
// 		err := c.Collection(models.UserSubscriptionCollection).FindOne(ctx, bson.M{"userId": userId, "channelId": channelId}).Decode(subscription)
// 		return subscription, err
// 	}

// 	var stages []interface{}
// 	stages = append(stages, bson.M{"$match": bson.M{"userId": userId, "channelId": channelId}})
// 	if options.WithChannel {
// 		stages = append(stages, bson.M{
// 			"$lookup": bson.M{
// 				"from":         models.ChannelCollection,
// 				"localField":   "channelId",
// 				"foreignField": "eid",
// 				"as":           "channels",
// 			}})
// 	}
// 	if options.WithUser {
// 		stages = append(stages, bson.M{
// 			"$lookup": bson.M{
// 				"from":         models.UserCollection,
// 				"localField":   "userId",
// 				"foreignField": "eid",
// 				"as":           "users",
// 			},
// 		})
// 	}

// 	cur, err := c.Collection(models.UserSubscriptionCollection).Aggregate(ctx, stages)
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = cur.All(ctx, &subscription)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return subscription, nil
// }

// func (c *Client) GetSubscription(id primitive.ObjectID, opts ...SubscriptionGetOptions) (*models.UserSubscription, error) {
// 	var options SubscriptionGetOptions
// 	if len(opts) > 0 {
// 		options = opts[0]
// 	}

// 	subscription := &models.UserSubscription{}
// 	ctx, cancel := c.Ctx()
// 	defer cancel()

// 	if !options.WithChannel && !options.WithUser {
// 		cur, err := c.Collection(models.UserSubscriptionCollection).Find(ctx, bson.M{"_id": id})
// 		if err != nil {
// 			return nil, err
// 		}
// 		return subscription, cur.All(ctx, &subscription)
// 	}

// 	var stages []interface{}
// 	stages = append(stages, bson.M{"$match": bson.M{"_id": id}})
// 	if options.WithChannel {
// 		stages = append(stages, bson.M{
// 			"$lookup": bson.M{
// 				"from":         models.ChannelCollection,
// 				"localField":   "channelId",
// 				"foreignField": "eid",
// 				"as":           "channels",
// 			}})
// 	}
// 	if options.WithUser {
// 		stages = append(stages, bson.M{
// 			"$lookup": bson.M{
// 				"from":         models.UserCollection,
// 				"localField":   "userId",
// 				"foreignField": "eid",
// 				"as":           "users",
// 			},
// 		})
// 	}

// 	cur, err := c.Collection(models.UserSubscriptionCollection).Aggregate(ctx, stages)
// 	if err != nil {
// 		return nil, err
// 	}
// 	err = cur.All(ctx, &subscription)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return subscription, nil
// }

// func (c *Client) DeleteSubscription(userId primitive.ObjectID, channelId string) error {
// 	ctx, cancel := c.Ctx()
// 	defer cancel()

// 	_, err := c.Collection(models.UserSubscriptionCollection).DeleteOne(ctx, bson.M{"userId": userId, "channelId": channelId})
// 	if errors.Is(mongo.ErrNoDocuments, err) {
// 		return nil
// 	}
// 	return err
// }

// func (c *Client) UpdateSubscription(sub *models.UserSubscription) error {
// 	sub.Prepare()

// 	ctx, cancel := c.Ctx()
// 	defer cancel()

// 	_, err := c.Collection(models.UserSubscriptionCollection).UpdateOne(ctx, bson.M{"_id": sub.ID}, bson.M{"$set": sub})
// 	return err
// }
