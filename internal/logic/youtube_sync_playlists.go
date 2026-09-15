package logic

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/pkg/errors"
	ytv3 "google.golang.org/api/youtube/v3"
)

type youtubePlaylistSource struct {
	state       *models.YoutubeSyncTarget
	playlist    *models.Playlist // nil denotes the home feed
	title       string
	description string
	feed        []string
}

func (s *YouTubeSyncService) playlistSources(ctx context.Context, account *models.YoutubeSyncAccount, feed []string) ([]youtubePlaylistSource, error) {
	states, err := s.db.ListYouTubeSyncTargets(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	bySource := make(map[string]*models.YoutubeSyncTarget, len(states))
	for _, state := range states {
		bySource[state.SourceID] = state
	}
	stateFor := func(sourceID string) *models.YoutubeSyncTarget {
		if state := bySource[sourceID]; state != nil {
			return state
		}
		return &models.YoutubeSyncTarget{AccountID: account.ID, SourceID: sourceID}
	}
	feedState := stateFor("feed")
	// Reuse the existing feed destination when upgrading to multiple playlists.
	if feedState.PlaylistID == "" && account.PlaylistID.Valid {
		feedState.PlaylistID = strings.TrimSpace(account.PlaylistID.String)
		feedState.Title = youtubeSyncPlaylistName
		feedState.Description = youtubeSyncPlaylistDescription
	}
	sources := []youtubePlaylistSource{{
		state: feedState, title: youtubeSyncPlaylistName,
		description: youtubeSyncPlaylistDescription, feed: feed,
	}}
	playlists, err := s.db.GetUserPlaylists(ctx, account.UserID)
	if err != nil {
		return nil, err
	}
	watchLater, err := s.db.GetPlaylistBySlug(ctx, account.UserID, WatchLaterSlug)
	if err != nil && !database.IsErrNotFound(err) {
		return nil, err
	}
	if err == nil {
		playlists = append(playlists, watchLater)
	}
	for _, playlist := range playlists {
		title := []rune("Feedlr: " + playlist.Name)
		if len(title) > 150 {
			title = title[:150]
		}
		description := youtubeSyncPlaylistDescription
		if playlist.Description != "" {
			description += "\n\n" + playlist.Description
		}
		// YouTube limits playlist descriptions to 5,000 bytes.
		if len(description) > 5000 {
			description = strings.ToValidUTF8(description[:5000], "")
		}
		sources = append(sources, youtubePlaylistSource{
			state: stateFor("playlist:" + playlist.ID), playlist: playlist,
			title: string(title), description: description,
		})
	}
	// Oldest attempt first gives every playlist a turn, even when the feed
	// changes constantly or another playlist repeatedly fails.
	slices.SortStableFunc(sources, func(a, b youtubePlaylistSource) int {
		if cmp := a.state.LastAttemptAt.Compare(b.state.LastAttemptAt); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.state.SourceID, b.state.SourceID)
	})
	return sources, nil
}

func (s *YouTubeSyncService) syncPlaylists(ctx context.Context, service *ytv3.Service, account *models.YoutubeSyncAccount, feed []string) error {
	sources, err := s.playlistSources(ctx, account, feed)
	if err != nil {
		return err
	}
	callsLeft := s.maxExpensiveCallsPerSync
	var failures []string
	for _, source := range sources {
		if callsLeft <= 0 {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		source.state.LastAttemptAt = time.Now().UTC()
		if err := s.db.UpsertYouTubeSyncTarget(ctx, source.state); err != nil {
			return err
		}
		if err := s.syncPlaylistSource(ctx, service, account, source, &callsLeft); err != nil {
			failures = append(failures, source.title+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, " | "))
	}
	return nil
}

func (s *YouTubeSyncService) playlistSourceVideos(ctx context.Context, source youtubePlaylistSource) ([]string, error) {
	if source.playlist == nil {
		return source.feed, nil
	}
	opts := []database.PlaylistItemQuery{database.PlaylistItem.WithVideo()}
	if source.playlist.Slug != WatchLaterSlug || !source.playlist.System {
		opts = append(opts, database.PlaylistItem.OrderByPosition())
	}
	items, err := s.db.GetPlaylistItems(ctx, source.playlist.ID, opts...)
	if err != nil {
		return nil, err
	}
	var desired []string
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.VideoID == "" || seen[item.VideoID] || item.R == nil || item.R.Video == nil {
			continue
		}
		video := item.R.Video
		if video.Type == string(youtube.VideoTypePodcastEpisode) || video.Private {
			continue
		}
		seen[item.VideoID] = true
		desired = append(desired, item.VideoID)
	}
	return desired, nil
}

func (s *YouTubeSyncService) syncPlaylistSource(ctx context.Context, service *ytv3.Service, account *models.YoutubeSyncAccount, source youtubePlaylistSource, callsLeft *int) error {
	desired, err := s.playlistSourceVideos(ctx, source)
	if err != nil {
		return err
	}
	state := source.state
	create := func() error {
		if *callsLeft <= 0 {
			return errors.New("no write calls left to create playlist")
		}
		*callsLeft-- // Failed writes also consume the run's budget.
		id, err := createYouTubeSyncPlaylist(ctx, service, source.title, source.description)
		if err != nil {
			return err
		}
		state.PlaylistID, state.Title, state.Description = id, source.title, source.description
		if err := s.db.UpsertYouTubeSyncTarget(ctx, state); err != nil {
			return err
		}
		return nil
	}
	var remote []playlistRemoteItem
	if state.PlaylistID == "" {
		if err := create(); err != nil {
			return err
		}
	} else {
		remote, err = listPlaylistItemsWithRetry(ctx, service, state.PlaylistID, 50)
		if isYouTubePlaylistNotFound(err) {
			err = create()
			remote = nil
		}
		if err != nil {
			return err
		}
	}
	// Keep the settings link compatible, including after remote deletion.
	if source.playlist == nil && state.PlaylistID != account.PlaylistID.String {
		if err := s.db.UpdateYouTubeSyncPlaylistID(ctx, account.UserID, state.PlaylistID); err != nil {
			return err
		}
	}
	if *callsLeft > 0 && (state.Title != source.title || state.Description != source.description) {
		*callsLeft--
		_, err := service.Playlists.Update([]string{"snippet"}, &ytv3.Playlist{
			Id:      state.PlaylistID,
			Snippet: &ytv3.PlaylistSnippet{Title: source.title, Description: source.description},
		}).Context(ctx).Do()
		metrics.ObserveYouTubeAPICall("playlist_sync", "update_playlist", err)
		if err != nil {
			return err
		}
		state.Title, state.Description = source.title, source.description
		if err := s.db.UpsertYouTubeSyncTarget(ctx, state); err != nil {
			return err
		}
	}
	plan := buildPlaylistSyncPlan(desired, remote, *callsLeft)
	for _, itemID := range plan.ToDelete {
		*callsLeft--
		if err := deletePlaylistItem(ctx, service, itemID); err != nil {
			return err
		}
	}
	for _, add := range plan.ToAdd {
		*callsLeft--
		if err := insertVideoIntoPlaylist(ctx, service, state.PlaylistID, add.VideoID, add.Position); err != nil {
			// Later insert positions assume every earlier insert succeeded.
			return err
		}
	}
	if len(plan.ToAdd) != 0 || len(plan.ToDelete) != 0 {
		return nil // Re-read positions before reordering on a later run.
	}
	return reorderYouTubePlaylist(ctx, service, state.PlaylistID, desired, remote, callsLeft)
}

func reorderYouTubePlaylist(ctx context.Context, service *ytv3.Service, playlistID string, desired []string, remote []playlistRemoteItem, callsLeft *int) error {
	work := slices.Clone(remote)
	for position, videoID := range desired {
		if *callsLeft <= 0 || position >= len(work) {
			break
		}
		index := slices.IndexFunc(work[position:], func(item playlistRemoteItem) bool { return item.VideoID == videoID })
		if index <= 0 {
			continue
		}
		index += position
		item := work[index]
		*callsLeft--
		_, err := service.PlaylistItems.Update([]string{"snippet"}, &ytv3.PlaylistItem{
			Id: item.ItemID,
			Snippet: &ytv3.PlaylistItemSnippet{
				PlaylistId: playlistID, Position: int64(position),
				ResourceId:      &ytv3.ResourceId{Kind: "youtube#video", VideoId: videoID},
				ForceSendFields: []string{"Position"},
			},
		}).Context(ctx).Do()
		metrics.ObserveYouTubeAPICall("playlist_sync", "update_playlist_item", err)
		if err != nil {
			return err
		}
		copy(work[position+1:index+1], work[position:index])
		work[position] = item
	}
	return nil
}
