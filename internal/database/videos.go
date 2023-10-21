package database

import (
	"context"
	"errors"

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
	Duration    int
	Description string
	Thumbnail   string
}

func (c *Client) NewVideo(channel string, videos ...VideoCreateModel) ([]db.VideoModel, error) {
	cl := db.Video.Channel.Link(db.Channel.ID.Equals(channel))
	var created []db.VideoModel

	for _, vid := range videos {
		v, err := c.p.Video.CreateOne(db.Video.ID.Set(vid.ID), db.Video.URL.Set(vid.URL), db.Video.Title.Set(vid.Title), db.Video.Description.Set(vid.Description), cl, db.Video.Thumbnail.Set(vid.Thumbnail), db.Video.Duration.Set(vid.Duration)).Exec(context.TODO())
		if err != nil {
			return nil, err
		}
		created = append(created, *v)
	}
	return created, nil
}

func (c *Client) GetUserVideoView(user, video string) (*db.VideoViewModel, error) {
	view, err := c.p.VideoView.FindFirst(db.VideoView.UserID.Equals(user), db.VideoView.VideoID.Equals(video)).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	return view, nil
}

func (c *Client) GetAllUserViews(user string) ([]db.VideoViewModel, error) {
	views, err := c.p.VideoView.FindMany(db.VideoView.UserID.Equals(user)).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	return views, nil
}

func (c *Client) UpsertView(user, video string, progress int) (*db.VideoViewModel, error) {
	view, err := c.p.VideoView.FindFirst(db.VideoView.UserID.Equals(user), db.VideoView.VideoID.Equals(video)).Exec(context.Background())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			view, err = c.p.VideoView.CreateOne(db.VideoView.User.Link(db.User.ID.Equals(user)), db.VideoView.Video.Link(db.Video.ID.Equals(video)), db.VideoView.Progress.Set(progress)).Exec(context.Background())
			if err != nil {
				return nil, err
			}
			return view, nil
		}
		return nil, err
	}

	view, err = c.p.VideoView.FindUnique(db.VideoView.ID.Equals(view.ID)).Update(db.VideoView.Progress.Set(progress)).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	return view, nil
}
