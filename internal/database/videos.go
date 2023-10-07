package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) GetVideoByID(id string) (*db.VideoModel, error) {
	video, err := c.p.Video.FindFirst(db.Video.ID.Equals(id)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return video, nil
}

func (c *Client) GetVideosByChannelID(limit int, channelIds ...string) ([]db.VideoModel, error) {
	query := c.p.Video.FindMany(db.Video.ChannelID.In(channelIds)).OrderBy(db.Video.CreatedAt.Order(db.SortOrderDesc))
	videos, err := query.Exec(context.TODO())
	if err != nil {
		return nil, err
	}

	return videos, nil
}

func (c *Client) GetVideos(id string, limit int) ([]db.VideoModel, error) {
	query := c.p.Video.FindMany(db.Video.ChannelID.Equals(id)).OrderBy(db.Video.CreatedAt.Order(db.SortOrderDesc))
	if limit > 0 {
		query = query.Take(limit)
	}

	videos, err := query.Exec(context.TODO())
	if err != nil {
		return nil, err
	}

	return videos, nil
}

type VideoCreateModel struct {
	ID          string
	URL         string
	Title       string
	Description string
	Thumbnail   string
}

func (c *Client) NewVideo(channel string, videos ...VideoCreateModel) ([]db.VideoModel, error) {
	cl := db.Video.Channel.Link(db.Channel.ID.Equals(channel))
	var created []db.VideoModel

	for _, vid := range videos {
		v, err := c.p.Video.CreateOne(db.Video.ID.Set(vid.ID), db.Video.URL.Set(vid.URL), db.Video.Title.Set(vid.Title), db.Video.Description.Set(vid.Description), cl, db.Video.Thumbnail.Set(vid.Thumbnail)).Exec(context.TODO())
		if err != nil {
			return nil, err
		}
		created = append(created, *v)
	}
	return created, nil
}
