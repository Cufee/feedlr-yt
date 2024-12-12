package youtube

import (
	"errors"
	"sort"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/youtube/v3"
)

type PlayListItemWithDetails struct {
	*youtube.PlaylistItem
	Duration int
}

func (c *client) GetChannelUploadPlaylistID(channelId string) (string, error) {
	playlists, err := c.service.Channels.List([]string{"id", "contentDetails"}).Id(channelId).Fields("items(contentDetails/relatedPlaylists/uploads)").Do()
	if err != nil {
		return "", errors.Join(errors.New("GetChannelUploadPlaylistID.youtube.service.Channels.List"), err)
	}

	if len(playlists.Items) <= 0 {
		return "", errors.New("GetChannelUploadPlaylistID.youtube.service.Channels.List: no channels found")
	}

	return playlists.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func (c *client) GetPlaylistVideos(playlistId string, uploadedAfter time.Time, limit int, skipVideoIds ...string) ([]Video, error) {
	if playlistId == "" {
		return nil, errors.New("playlist id cannot be blank")
	}
	if limit < 1 {
		limit = 3
	}

	res, err := c.service.PlaylistItems.List([]string{"id", "snippet"}).PlaylistId(playlistId).MaxResults(50).Do() // https://developers.google.com/youtube/v3/docs/playlists/list#parameters
	if err != nil {
		return nil, errors.Join(errors.New("GetPlaylistVideos.youtube.service.PlaylistItems.List"), err)
	}

	var group errgroup.Group
	group.SetLimit(3)

	var videoDetails = make(chan *VideoDetails, 50)

	for _, item := range res.Items {
		if slices.Contains(skipVideoIds, item.Snippet.ResourceId.VideoId) {
			continue
		}

		group.Go(func() error {
			if len(videoDetails) > limit+1 { // +1 just in case
				return nil
			}

			publishedAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			if publishedAt.Before(uploadedAfter) {
				return nil
			}

			details, err := c.GetVideoPlayerDetails(item.Snippet.ResourceId.VideoId, 2)
			if err != nil {
				return err
			}
			if details == nil || details.Type == VideoTypeShort {
				return nil
			}

			details.Title = item.Snippet.Title
			details.ChannelID = item.Snippet.ChannelId
			details.Description = item.Snippet.Description
			details.PublishedAt = publishedAt
			videoDetails <- details
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	close(videoDetails)

	var videos []Video
	for item := range videoDetails {
		videos = append(videos, item.Video)
	}

	sort.Slice(videos, func(i, j int) bool {
		return videos[i].PublishedAt.After(videos[j].PublishedAt)
	})

	if len(videos) > limit {
		videos = videos[:limit]
	}

	// Reverse slice to get videos in descending order
	sort.Slice(videos, func(i, j int) bool {
		return true
	})

	return videos, nil
}
