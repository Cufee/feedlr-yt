package youtube

import (
	"errors"
	"log"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"
)

type Channel struct {
	ID          string
	URL         string
	Title       string
	Thumbnail   string
	Description string
}
type Video struct {
	Type        VideoType
	ID          string
	URL         string
	Title       string
	Duration    int
	Thumbnail   string
	PublishedAt string
	Description string
}

func (c *client) SearchChannels(query string, limit int) ([]Channel, error) {
	if limit < 1 {
		limit = 3
	}
	res, err := c.service.Search.List([]string{"id", "snippet"}).Q(query).Type("channel").MaxResults((int64(limit))).Do()
	if err != nil {
		return nil, errors.Join(errors.New("SearchChannels.youtube.service.Search.List"), err)
	}

	var channels []Channel
	for _, item := range res.Items {
		channels = append(channels, Channel{
			ID:          item.Id.ChannelId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Thumbnail:   item.Snippet.Thumbnails.Default.Url,
			URL:         c.BuildChannelURL(item.Id.ChannelId),
		})
	}

	return channels, nil
}

func (c *client) GetChannel(channelID string) (*Channel, error) {
	res, err := c.service.Channels.List([]string{"id", "snippet"}).Id(channelID).Do()
	if err != nil {
		return nil, errors.Join(errors.New("GetChannel.youtube.service.Channels.List"), err)
	}

	if len(res.Items) <= 0 {
		return nil, errors.New("GetChannel.youtube.service.Channels.List: no channels found")
	}

	var channel Channel
	channel.ID = res.Items[0].Id
	channel.Title = res.Items[0].Snippet.Title
	channel.Thumbnail = res.Items[0].Snippet.Thumbnails.Medium.Url
	channel.Description = res.Items[0].Snippet.Description

	return &channel, nil
}

func (c *client) GetChannelVideos(channelID string, limit int, skipVideoIds ...string) ([]Video, error) {
	res, err := c.service.Search.List([]string{"snippet", "id"}).ChannelId(channelID).Order("date").MaxResults(50).Do()
	if err != nil {
		return nil, err
	}

	var group errgroup.Group
	group.SetLimit(1) // this is required for the limit take work

	videos := make(chan Video, len(res.Items))
	for _, item := range res.Items {
		if slices.Contains(skipVideoIds, item.Id.VideoId) {
			continue
		}

		group.Go(func() error {
			video := Video{
				ID:          item.Id.VideoId,
				URL:         c.BuildVideoEmbedURL(item.Id.VideoId),
				Title:       item.Snippet.Title,
				Description: item.Snippet.Description,
				PublishedAt: item.Snippet.PublishedAt,
				Thumbnail:   c.BuildVideoThumbnailURL(item.Id.VideoId),
			}

			if limit > 0 && len(videos) > limit {
				return nil
			}

			details, err := c.GetVideoPlayerDetails(video.ID)
			if err != nil {
				videos <- video
				log.Printf("Failed to get video details for %s\n", video.ID)
				return nil
			}

			if details == nil || details.Type == VideoTypeShort {
				return nil
			}

			video.Type = details.Type
			video.Duration = details.Duration
			videos <- video
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	close(videos)

	var videosSlice []Video
	for v := range videos {
		if len(videosSlice) >= limit {
			break
		}
		videosSlice = append(videosSlice, v)
	}

	slices.SortFunc(videosSlice, func(a, b Video) int {
		return strings.Compare(b.PublishedAt, a.PublishedAt)
	})
	return videosSlice, nil
}
