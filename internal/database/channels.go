package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) GetAllChannels() ([]db.ChannelModel, error) {
	channels, err := c.p.Channel.FindMany().With(db.Channel.Videos.Fetch()).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) GetChannel(channelId string) (*db.ChannelModel, error) {
	channel, err := c.p.Channel.FindUnique(db.Channel.ID.Equals(channelId)).With(db.Channel.Videos.Fetch()).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (c *Client) GetChannelsByID(channelIds ...string) ([]db.ChannelModel, error) {
	channels, err := c.p.Channel.FindMany(db.Channel.ID.In(channelIds)).With(db.Channel.Videos.Fetch()).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return channels, nil
}

func (c *Client) GetVideosByChannelID(channelIds ...string) ([]db.ChannelVideoModel, error) {
	videos, err := c.p.ChannelVideo.FindMany(db.ChannelVideo.ChannelID.In(channelIds)).OrderBy(db.ChannelVideo.CreatedAt.Order(db.SortOrderDesc)).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return videos, nil
}

type ChannelCreateModel struct {
	ID          string
	URL         string
	Title       string
	Description string
	Thumbnail   string
}

func (c *Client) NewChannel(ch ChannelCreateModel) (*db.ChannelModel, error) {
	return c.p.Channel.CreateOne(db.Channel.ID.Set(ch.ID), db.Channel.URL.Set(ch.URL), db.Channel.Title.Set(ch.Title), db.Channel.Description.Set(ch.Description), db.Channel.Thumbnail.Set(ch.Thumbnail)).Exec(context.TODO())
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
