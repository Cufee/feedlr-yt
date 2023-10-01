package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) GetVideoByID(id string) (*db.ChannelVideoModel, error) {
	video, err := c.p.ChannelVideo.FindFirst(db.ChannelVideo.ID.Equals(id)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return video, nil
}

func (c *Client) GetVideosByChannelID(limit int, channelIds ...string) ([]db.ChannelVideoModel, error) {
	query := c.p.ChannelVideo.FindMany(db.ChannelVideo.ChannelID.In(channelIds)).OrderBy(db.ChannelVideo.CreatedAt.Order(db.SortOrderDesc))
	videos, err := query.Exec(context.TODO())
	if err != nil {
		return nil, err
	}

	return videos, nil
}

func (c *Client) GetChannelVideos(id string, limit int) ([]db.ChannelVideoModel, error) {
	query := c.p.ChannelVideo.FindMany(db.ChannelVideo.ChannelID.Equals(id)).OrderBy(db.ChannelVideo.CreatedAt.Order(db.SortOrderDesc))
	if limit > 0 {
		query = query.Take(limit)
	}

	videos, err := query.Exec(context.TODO())
	if err != nil {
		return nil, err
	}

	return videos, nil
}

type ChannelVideoCreateModel struct {
	ID          string
	URL         string
	Title       string
	Description string
	Thumbnail   string
}

func (c *Client) NewChannelVideo(channel string, videos ...ChannelVideoCreateModel) ([]db.ChannelVideoModel, error) {
	cl := db.ChannelVideo.Channel.Link(db.Channel.ID.Equals(channel))
	var created []db.ChannelVideoModel

	for _, vid := range videos {
		v, err := c.p.ChannelVideo.CreateOne(db.ChannelVideo.ID.Set(vid.ID), db.ChannelVideo.URL.Set(vid.URL), db.ChannelVideo.Title.Set(vid.Title), db.ChannelVideo.Description.Set(vid.Description), cl, db.ChannelVideo.Thumbnail.Set(vid.Thumbnail)).Exec(context.TODO())
		if err != nil {
			return nil, err
		}
		created = append(created, *v)
	}
	return created, nil
}
