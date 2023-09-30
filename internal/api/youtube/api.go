package youtube

import "errors"

func (c *YouTubeClient) SearchChannels(query string, limit int) ([]Channel, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).Q(query).Type("channel").MaxResults((int64(limit))).Do()
	if err != nil {
		return nil, err
	}

	var channels []Channel
	for _, item := range res.Items {
		channels = append(channels, Channel{
			ID:          item.Id.ChannelId,
			Title:       item.Snippet.Title,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			Description: item.Snippet.Description,
		})
	}

	return channels, nil
}

func (c *YouTubeClient) GetChannel(channelID string) (*Channel, error) {
	res, err := c.service.Channels.List([]string{"id", "snippet"}).Id(channelID).Do()
	if err != nil {
		return nil, err
	}

	if len(res.Items) <= 0 {
		return nil, errors.New("channel not found")
	}

	var channel Channel
	channel.ID = res.Items[0].Id
	channel.Title = res.Items[0].Snippet.Title
	channel.Thumbnail = res.Items[0].Snippet.Thumbnails.Default.Url

	return &channel, nil
}

func (c *YouTubeClient) GetChannelVideos(channelID string, limit int) ([]Video, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).ChannelId(channelID).Type("video").MaxResults(int64(limit)).Do()
	if err != nil {
		return nil, err
	}

	var videos []Video
	for _, item := range res.Items {
		videos = append(videos, Video{
			ID:          item.Id.VideoId,
			Title:       item.Snippet.Title,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			Description: item.Snippet.Description,
		})
	}

	return videos, nil
}
