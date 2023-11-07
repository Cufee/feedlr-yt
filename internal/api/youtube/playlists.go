package youtube

import (
	"errors"
	"sort"
	"sync"

	"golang.org/x/exp/slices"
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

func (c *client) GetPlaylistVideos(playlistId string, limit int, sipVideoIds ...string) ([]Video, error) {
	if limit < 1 {
		limit = 3
	}

	res, err := c.service.PlaylistItems.List([]string{"id", "snippet"}).PlaylistId(playlistId).MaxResults(int64(limit * 3)).Do()
	if err != nil {
		return nil, errors.Join(errors.New("GetPlaylistVideos.youtube.service.PlaylistItems.List"), err)
	}

	var wg sync.WaitGroup
	var errChan = make(chan error, len(res.Items))
	var videoDetails = make(chan *VideoDetails, len(res.Items))

	for _, item := range res.Items {
		if slices.Contains(sipVideoIds, item.Snippet.ResourceId.VideoId) {
			continue
		}
		wg.Add(1)
		go func(item *youtube.PlaylistItem) {
			defer wg.Done()
			details, err := c.GetVideoPlayerDetails(item.Snippet.ResourceId.VideoId)
			if err != nil {
				errChan <- errors.Join(errors.New("GetPlaylistVideos.youtube.GetVideoPlayerDetails"), err)
				return
			}
			if details.Type == VideoTypeShort {
				// This app doesn't support shorts at all by design
				return
			}
			details.PublishedAt = item.Snippet.PublishedAt
			videoDetails <- details
		}(item)
	}
	wg.Wait()
	close(errChan)
	close(videoDetails)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	var videos []Video
	for item := range videoDetails {
		videos = append(videos, item.Video)
	}

	sort.Slice(videos, func(i, j int) bool {
		return videos[i].PublishedAt > videos[j].PublishedAt
	})

	if len(videos) > limit {
		videos = videos[:limit]
	}
	return videos, nil
}
