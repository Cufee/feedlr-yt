package database

import (
	"context"

	"github.com/byvko-dev/youtube-app/prisma/db"
)

func (c *Client) GetChannel(channelId string) (*db.ChannelModel, error) {
	channel, err := c.p.Channel.FindUnique(db.Channel.ID.Equals(channelId)).With(db.Channel.Videos.Fetch()).Exec(context.TODO())
	if err != nil {
		return nil, err
	}
	return channel, nil
}

func (c *Client) NewChannel(id, title, thumb, desc string) (*db.ChannelModel, error) {
	return c.p.Channel.CreateOne(db.Channel.ID.Set(id), db.Channel.Title.Set(title), db.Channel.Thumbnail.Set(thumb), db.Channel.Description.Set(desc)).Exec(context.TODO())
}
