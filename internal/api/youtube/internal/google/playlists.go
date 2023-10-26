package google

import (
	"errors"
	"sort"
	"sync"

	yt "github.com/cufee/feedlr-yt/internal/api/youtube/client"
	"golang.org/x/exp/slices"
	"google.golang.org/api/youtube/v3"
)

type PlayListItemWithDuration struct {
	*youtube.PlaylistItem
	Duration int
}

func (c *client) GetChannelUploadPlaylistID(channelId string) (string, error) {
	playlists, err := c.service.Channels.List([]string{"id", "contentDetails"}).Id(channelId).Fields("items(contentDetails/relatedPlaylists/uploads)").Do()
	if err != nil {
		return "", err
	}

	if len(playlists.Items) <= 0 {
		return "", errors.New("channel not found")
	}

	return playlists.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func (c *client) GetPlaylistVideos(playlistId string, limit int, sipVideoIds ...string) ([]yt.Video, error) {
	if limit < 1 {
		limit = 3
	}

	res, err := c.service.PlaylistItems.List([]string{"id", "snippet"}).PlaylistId(playlistId).MaxResults(int64(limit * 3)).Do()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var errChan = make(chan error, len(res.Items))
	var validVideos = make(chan PlayListItemWithDuration, len(res.Items))

	for _, item := range res.Items {
		if slices.Contains(sipVideoIds, item.Snippet.ResourceId.VideoId) {
			continue
		}
		wg.Add(1)
		go func(item *youtube.PlaylistItem) {
			defer wg.Done()
			details, err := c.GetVideoPlayerDetails(item.Snippet.ResourceId.VideoId)
			if err != nil {
				errChan <- err
				return
			}
			if details.IsShort {
				return
			}
			validVideos <- PlayListItemWithDuration{
				PlaylistItem: item,
				Duration:     details.Duration,
			}
		}(item)
	}
	wg.Wait()
	close(errChan)
	close(validVideos)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	var validVideosSlice []PlayListItemWithDuration
	for item := range validVideos {
		validVideosSlice = append(validVideosSlice, item)
	}

	sort.Slice(validVideosSlice, func(i, j int) bool {
		return validVideosSlice[i].Snippet.PublishedAt > validVideosSlice[j].Snippet.PublishedAt
	})

	var videos []yt.Video
	for _, item := range validVideosSlice {
		videos = append(videos, yt.Video{
			ID:          item.Snippet.ResourceId.VideoId,
			Title:       item.Snippet.Title,
			Duration:    item.Duration,
			Description: item.Snippet.Description,
			Thumbnail:   item.Snippet.Thumbnails.High.Url,
			URL:         c.buildVideoEmbedURL(item.Snippet.ResourceId.VideoId),
		})
		if len(videos) >= limit {
			break
		}
	}

	return videos, nil
}
