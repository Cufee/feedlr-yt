package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

type ChannelGetOptions struct {
	WithVideos bool
}

func (c *Client) GetAllChannels(opts ...ChannelGetOptions) ([]db.ChannelModel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	query := c.p.Channel.FindMany()
	if options.WithVideos {
		query = query.With(db.Channel.Videos.Fetch())
	}
	return query.Exec(context.TODO())
}

func (c *Client) GetChannel(channelId string, opts ...ChannelGetOptions) (*db.ChannelModel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	query := c.p.Channel.FindUnique(db.Channel.ID.Equals(channelId))
	if options.WithVideos {
		query = query.With(db.Channel.Videos.Fetch())
	}
	return query.Exec(context.TODO())
}

func (c *Client) GetChannelsByID(channelIds []string, opts ...ChannelGetOptions) ([]db.ChannelModel, error) {
	var options ChannelGetOptions
	if len(opts) > 0 {
		options = opts[0]
	}

	query := c.p.Channel.FindMany(db.Channel.ID.In(channelIds))
	if options.WithVideos {
		query = query.With(db.Channel.Videos.Fetch())
	}
	return query.Exec(context.TODO())
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
