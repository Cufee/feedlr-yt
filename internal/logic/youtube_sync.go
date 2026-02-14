package logic

import (
	"context"
	stdErrors "errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/netproxy"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	ytv3 "google.golang.org/api/youtube/v3"
)

const (
	youTubeSyncOAuthScope = "https://www.googleapis.com/auth/youtube"

	youtubeSyncPlaylistName        = "Feedlr Sync"
	youtubeSyncPlaylistDescription = "Managed by Feedlr"
	youtubeSyncPlaylistSize        = 36
	youtubeSyncListRetryAttempts   = 4
)

var DefaultYouTubeSync *YouTubeSyncService

type YouTubeSyncService struct {
	db database.Client

	crypto      *youtubeSyncCrypto
	oauthConfig *oauth2.Config

	maxExpensiveCallsPerSync int
	maxUsersPerTick          int
}

func NewYouTubeSyncService(db database.Client) (*YouTubeSyncService, error) {
	service := &YouTubeSyncService{
		db: db,
		crypto: newYouTubeSyncCrypto(
			utils.MustGetEnv("YOUTUBE_SYNC_ENCRYPTION_SECRET"),
		),
		oauthConfig: &oauth2.Config{
			ClientID:     utils.MustGetEnv("YOUTUBE_OAUTH_CLIENT_ID"),
			ClientSecret: utils.MustGetEnv("YOUTUBE_OAUTH_CLIENT_SECRET"),
			RedirectURL:  utils.MustGetEnv("YOUTUBE_OAUTH_REDIRECT_URL"),
			Scopes:       []string{youTubeSyncOAuthScope},
			Endpoint:     google.Endpoint,
		},
		maxExpensiveCallsPerSync: 4,
		maxUsersPerTick:          100,
	}
	if service.maxExpensiveCallsPerSync <= 0 {
		return nil, errors.New("PLAYLIST_SYNC_MAX_EXPENSIVE_CALLS must be greater than 0")
	}
	if service.maxUsersPerTick <= 0 {
		return nil, errors.New("PLAYLIST_SYNC_MAX_USERS_PER_TICK must be greater than 0")
	}

	return service, nil
}

func (s *YouTubeSyncService) OAuthAuthURL(state string) string {
	return s.oauthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

func (s *YouTubeSyncService) CompleteOAuth(ctx context.Context, userID, code string) error {
	proxyHTTPClient, err := netproxy.NewYouTubeHTTPClient(0)
	if err != nil {
		return errors.Wrap(err, "failed to initialize youtube sync http client")
	}
	oauthCtx := context.WithValue(ctx, oauth2.HTTPClient, proxyHTTPClient)

	token, err := s.oauthConfig.Exchange(oauthCtx, code)
	metrics.ObserveYouTubeOAuthCall("playlist_sync_oauth_exchange", err)
	if err != nil {
		return errors.Wrap(err, "oauth exchange failed")
	}

	refreshToken := token.RefreshToken
	if refreshToken == "" {
		existing, err := s.db.GetYouTubeSyncAccountByUserID(ctx, userID)
		if err == nil {
			decrypted, derr := s.decryptRefreshToken(existing)
			if derr == nil {
				refreshToken = string(decrypted)
			}
		}
	}
	if refreshToken == "" {
		return errors.New("oauth did not return a refresh token")
	}

	encrypted, err := s.crypto.Encrypt([]byte(refreshToken), userID)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt refresh token")
	}

	err = s.db.UpsertYouTubeSyncCredentials(ctx, userID, encrypted, s.crypto.secretHash)
	metrics.ObserveYouTubeOAuthCall("playlist_sync_save_credentials", err)
	if err != nil {
		return errors.Wrap(err, "failed to persist oauth credentials")
	}

	return nil
}

func (s *YouTubeSyncService) Disconnect(ctx context.Context, userID string) error {
	err := s.db.DeleteYouTubeSyncAccount(ctx, userID)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	return nil
}

func (s *YouTubeSyncService) SetEnabled(ctx context.Context, userID string, enabled bool) error {
	err := s.db.SetYouTubeSyncAccountEnabled(ctx, userID, enabled)
	if err != nil {
		return err
	}
	return nil
}

func (s *YouTubeSyncService) Status(ctx context.Context, userID string) (types.YouTubeSyncStatusProps, error) {
	status := types.YouTubeSyncStatusProps{
		Available: true,
	}

	account, err := s.db.GetYouTubeSyncAccountByUserID(ctx, userID)
	if err != nil {
		if database.IsErrNotFound(err) {
			return status, nil
		}
		return status, err
	}

	status.Connected = true
	status.Enabled = account.SyncEnabled
	status.PlaylistID = account.PlaylistID.String
	status.LastError = account.LastError
	if account.LastSyncedAt.Valid {
		status.LastSyncedAt = account.LastSyncedAt.Time
	}
	return status, nil
}

func (s *YouTubeSyncService) RunSyncTick(ctx context.Context) error {
	accounts, err := s.db.ListEnabledYouTubeSyncAccounts(ctx, s.maxUsersPerTick)
	metrics.ObserveBackgroundTask("youtube_sync_list_accounts", err)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}

	for _, account := range accounts {
		runCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := s.syncUser(runCtx, account)
		metrics.ObserveBackgroundTask("youtube_sync_user", err)
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("userID", account.UserID).Msg("youtube sync failed")
		}
	}

	return nil
}

func (s *YouTubeSyncService) RunSyncForUser(ctx context.Context, userID string) error {
	account, err := s.db.GetYouTubeSyncAccountByUserID(ctx, userID)
	metrics.ObserveBackgroundTask("youtube_sync_load_user_account", err)
	if err != nil {
		return err
	}
	err = s.syncUser(ctx, account)
	metrics.ObserveBackgroundTask("youtube_sync_user", err)
	return err
}

func (s *YouTubeSyncService) syncUser(ctx context.Context, account *models.YoutubeSyncAccount) error {
	attemptedAt := time.Now().UTC()

	desired, latestPublishedAt, err := s.desiredVideosForUser(ctx, account.UserID)
	if err != nil {
		s.storeRunResult(ctx, account.UserID, account.LastFeedVideoPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
		return err
	}

	if account.LastFeedVideoPublishedAt.Valid && !latestPublishedAt.Valid {
		latestPublishedAt = account.LastFeedVideoPublishedAt
	}

	service, err := s.youtubeServiceForAccount(ctx, account)
	if err != nil {
		s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
		return err
	}

	expensiveCallsLeft := s.maxExpensiveCallsPerSync

	playlistID := ""
	if account.PlaylistID.Valid {
		playlistID = strings.TrimSpace(account.PlaylistID.String)
	}

	if playlistID == "" {
		if expensiveCallsLeft < 1 {
			err := errors.New("playlist missing but no write calls left in current sync budget")
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}

		playlistID, err = createYouTubeSyncPlaylist(ctx, service)
		if err != nil {
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}
		expensiveCallsLeft--
		if err := s.db.UpdateYouTubeSyncPlaylistID(ctx, account.UserID, playlistID); err != nil {
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}
	}

	remoteItems, err := listPlaylistItemsWithRetry(ctx, service, playlistID, 50)
	if err != nil && isYouTubePlaylistNotFound(err) {
		if expensiveCallsLeft < 1 {
			err = errors.Wrap(err, "playlist missing remotely and no write calls left in current sync budget")
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}

		playlistID, err = createYouTubeSyncPlaylist(ctx, service)
		if err != nil {
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}
		expensiveCallsLeft--
		if err := s.db.UpdateYouTubeSyncPlaylistID(ctx, account.UserID, playlistID); err != nil {
			s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
			return err
		}

		remoteItems, err = listPlaylistItemsWithRetry(ctx, service, playlistID, 50)
	}
	if err != nil {
		s.storeRunResult(ctx, account.UserID, latestPublishedAt, account.LastSyncedAt, attemptedAt, err.Error())
		return err
	}

	plan := buildPlaylistSyncPlan(desired, remoteItems, expensiveCallsLeft)
	if len(plan.ToAdd) == 0 && len(plan.ToDelete) == 0 {
		syncedAt := null.TimeFrom(time.Now().UTC())
		s.storeRunResult(ctx, account.UserID, latestPublishedAt, syncedAt, attemptedAt, "")
		return nil
	}

	var mutationErrors []string
	var firstMutationErr error

	for _, itemID := range plan.ToDelete {
		err := deletePlaylistItem(ctx, service, itemID)
		if err != nil {
			err = errors.Wrapf(err, "failed to delete playlist item %s", itemID)
			if firstMutationErr == nil {
				firstMutationErr = err
			}
			mutationErrors = append(mutationErrors, err.Error())
		}
	}

	if firstMutationErr != nil {
		lastErr := strings.Join(mutationErrors, " | ")
		if len(lastErr) > 4000 {
			lastErr = lastErr[:4000]
		}
		s.storeRunResult(ctx, account.UserID, account.LastFeedVideoPublishedAt, account.LastSyncedAt, attemptedAt, lastErr)
		return firstMutationErr
	}

	for _, add := range plan.ToAdd {
		err := insertVideoIntoPlaylist(ctx, service, playlistID, add.VideoID, add.Position)
		if err != nil {
			err = errors.Wrapf(err, "failed to insert video %s into playlist %s at position %d", add.VideoID, playlistID, add.Position)
			if firstMutationErr == nil {
				firstMutationErr = err
			}
			mutationErrors = append(mutationErrors, err.Error())
		}
	}

	if firstMutationErr != nil {
		lastErr := strings.Join(mutationErrors, " | ")
		if len(lastErr) > 4000 {
			lastErr = lastErr[:4000]
		}
		s.storeRunResult(ctx, account.UserID, account.LastFeedVideoPublishedAt, account.LastSyncedAt, attemptedAt, lastErr)
		return firstMutationErr
	}

	syncedAt := null.TimeFrom(time.Now().UTC())
	s.storeRunResult(ctx, account.UserID, latestPublishedAt, syncedAt, attemptedAt, "")
	return nil
}

func (s *YouTubeSyncService) desiredVideosForUser(ctx context.Context, userID string) ([]string, null.Time, error) {
	props, err := GetUserVideosProps(ctx, s.db, userID)
	if err != nil {
		return nil, null.Time{}, err
	}

	seen := make(map[string]bool)
	var desired []string
	var latest null.Time

	add := func(videos []types.VideoProps) {
		for _, video := range videos {
			if video.ID == "" || seen[video.ID] {
				continue
			}
			seen[video.ID] = true
			desired = append(desired, video.ID)

			if !video.PublishedAt.IsZero() && (!latest.Valid || video.PublishedAt.After(latest.Time)) {
				latest = null.TimeFrom(video.PublishedAt.UTC())
			}
			if len(desired) >= youtubeSyncPlaylistSize {
				return
			}
		}
	}

	add(props.New)
	if len(desired) < youtubeSyncPlaylistSize {
		add(props.Watched)
	}

	if len(desired) > youtubeSyncPlaylistSize {
		desired = desired[:youtubeSyncPlaylistSize]
	}
	return desired, latest, nil
}

func (s *YouTubeSyncService) decryptRefreshToken(account *models.YoutubeSyncAccount) ([]byte, error) {
	if account.EncSecretHash != s.crypto.secretHash {
		return nil, errors.New("encryption secret hash mismatch; reconnect required")
	}

	decrypted, err := s.crypto.Decrypt(account.RefreshTokenEnc, account.UserID)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

func (s *YouTubeSyncService) youtubeServiceForAccount(ctx context.Context, account *models.YoutubeSyncAccount) (*ytv3.Service, error) {
	refreshToken, err := s.decryptRefreshToken(account)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range refreshToken {
			refreshToken[i] = 0
		}
	}()

	proxyHTTPClient, err := netproxy.NewYouTubeHTTPClient(0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize youtube sync http client")
	}
	oauthCtx := context.WithValue(ctx, oauth2.HTTPClient, proxyHTTPClient)

	tokenSource := s.oauthConfig.TokenSource(oauthCtx, &oauth2.Token{
		RefreshToken: string(refreshToken),
	})

	httpClient := oauth2.NewClient(oauthCtx, tokenSource)
	service, err := ytv3.NewService(oauthCtx, option.WithHTTPClient(httpClient))
	metrics.ObserveYouTubeAPICall("playlist_sync", "new_service", err)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (s *YouTubeSyncService) storeRunResult(
	ctx context.Context,
	userID string,
	lastFeedPublishedAt null.Time,
	lastSyncedAt null.Time,
	attemptedAt time.Time,
	lastErr string,
) {
	err := s.db.UpdateYouTubeSyncRunResult(ctx, userID, database.YouTubeSyncRunResult{
		LastFeedVideoPublishedAt: lastFeedPublishedAt,
		LastSyncedAt:             lastSyncedAt,
		LastSyncAttemptAt:        attemptedAt,
		LastError:                lastErr,
	})
	if err != nil {
		log.Warn().Err(err).Str("userID", userID).Msg("failed to persist youtube sync run result")
	}
}

type playlistRemoteItem struct {
	ItemID   string
	VideoID  string
	Position int64
}

type playlistSyncPlan struct {
	ToAdd    []playlistAddOperation
	ToDelete []string
}

type playlistAddOperation struct {
	VideoID  string
	Position int64
}

type playlistWorkItem struct {
	VideoID string
}

func buildPlaylistSyncPlan(desired []string, remote []playlistRemoteItem, maxExpensiveCalls int) playlistSyncPlan {
	if maxExpensiveCalls <= 0 {
		return playlistSyncPlan{}
	}

	desiredSet := make(map[string]bool, len(desired))
	for _, id := range desired {
		desiredSet[id] = true
	}

	seenRemote := make(map[string]bool, len(remote))
	var removeCandidates []playlistRemoteItem
	for _, item := range remote {
		if item.VideoID == "" {
			removeCandidates = append(removeCandidates, item)
			continue
		}
		if !desiredSet[item.VideoID] {
			removeCandidates = append(removeCandidates, item)
			continue
		}
		if seenRemote[item.VideoID] {
			removeCandidates = append(removeCandidates, item)
			continue
		}
		seenRemote[item.VideoID] = true
	}

	var toAddCandidates []string
	for _, id := range desired {
		if !seenRemote[id] {
			toAddCandidates = append(toAddCandidates, id)
		}
	}

	slices.SortFunc(removeCandidates, func(a, b playlistRemoteItem) int {
		switch {
		case a.Position > b.Position:
			return -1
		case a.Position < b.Position:
			return 1
		default:
			return 0
		}
	})

	insertBudget := maxExpensiveCalls / 2
	deleteBudget := maxExpensiveCalls - insertBudget

	addCount := min(insertBudget, len(toAddCandidates))
	deleteCount := min(deleteBudget, len(removeCandidates))
	used := addCount + deleteCount

	// Borrow unused budget, preferring inserts (most recent content first).
	remaining := maxExpensiveCalls - used
	if remaining > 0 {
		extraAdds := min(remaining, len(toAddCandidates)-addCount)
		addCount += extraAdds
		remaining -= extraAdds
	}
	if remaining > 0 {
		extraDeletes := min(remaining, len(removeCandidates)-deleteCount)
		deleteCount += extraDeletes
	}

	selectedAdds := slices.Clone(toAddCandidates[:addCount])
	plan := playlistSyncPlan{}
	deletedItemIDs := make(map[string]bool, deleteCount)
	for _, item := range removeCandidates[:deleteCount] {
		if item.ItemID == "" {
			continue
		}
		plan.ToDelete = append(plan.ToDelete, item.ItemID)
		deletedItemIDs[item.ItemID] = true
	}
	remoteForInsert := remote
	if len(deletedItemIDs) > 0 {
		remoteForInsert = filterRemoteItems(remote, deletedItemIDs)
	}
	plan.ToAdd = buildPlaylistAddOperations(desired, remoteForInsert, selectedAdds)

	return plan
}

func buildPlaylistAddOperations(desired []string, remote []playlistRemoteItem, selectedAdds []string) []playlistAddOperation {
	if len(selectedAdds) == 0 {
		return nil
	}

	selectedSet := make(map[string]bool, len(selectedAdds))
	for _, id := range selectedAdds {
		selectedSet[id] = true
	}

	work := make([]playlistWorkItem, 0, len(remote)+len(selectedAdds))
	present := make(map[string]bool, len(remote)+len(selectedAdds))
	for _, item := range remote {
		work = append(work, playlistWorkItem{VideoID: item.VideoID})
		if item.VideoID != "" {
			present[item.VideoID] = true
		}
	}

	var ops []playlistAddOperation
	for i := len(desired) - 1; i >= 0; i-- {
		videoID := desired[i]
		if !selectedSet[videoID] || present[videoID] {
			continue
		}

		position := len(work)
		for j := i + 1; j < len(desired); j++ {
			anchorPosition := indexOfPlaylistVideo(work, desired[j])
			if anchorPosition >= 0 {
				position = anchorPosition
				break
			}
		}

		ops = append(ops, playlistAddOperation{
			VideoID:  videoID,
			Position: int64(position),
		})

		work = slices.Insert(work, position, playlistWorkItem{VideoID: videoID})
		present[videoID] = true
	}

	return ops
}

func indexOfPlaylistVideo(work []playlistWorkItem, videoID string) int {
	for i := range work {
		if work[i].VideoID == videoID {
			return i
		}
	}
	return -1
}

func filterRemoteItems(remote []playlistRemoteItem, excludedItemIDs map[string]bool) []playlistRemoteItem {
	filtered := make([]playlistRemoteItem, 0, len(remote))
	for _, item := range remote {
		if item.ItemID != "" && excludedItemIDs[item.ItemID] {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func createYouTubeSyncPlaylist(ctx context.Context, service *ytv3.Service) (string, error) {
	playlist, err := service.Playlists.Insert([]string{"snippet", "status"}, &ytv3.Playlist{
		Snippet: &ytv3.PlaylistSnippet{
			Title:       youtubeSyncPlaylistName,
			Description: youtubeSyncPlaylistDescription,
		},
		Status: &ytv3.PlaylistStatus{
			PrivacyStatus: "private",
		},
	}).Context(ctx).Do()
	metrics.ObserveYouTubeAPICall("playlist_sync", "create_playlist", err)
	if err != nil {
		return "", err
	}

	if playlist == nil || playlist.Id == "" {
		return "", errors.New("playlist creation returned empty id")
	}
	return playlist.Id, nil
}

func listPlaylistItems(ctx context.Context, service *ytv3.Service, playlistID string, maxResults int64) ([]playlistRemoteItem, error) {
	call := service.PlaylistItems.List([]string{"id", "snippet"}).PlaylistId(playlistID)
	if maxResults > 0 {
		call = call.MaxResults(maxResults)
	}

	result, err := call.Context(ctx).Do()
	metrics.ObserveYouTubeAPICall("playlist_sync", "list_playlist_items", err)
	if err != nil {
		return nil, err
	}

	var items []playlistRemoteItem
	for _, item := range result.Items {
		remote := playlistRemoteItem{
			ItemID: item.Id,
		}
		if item.Snippet != nil {
			remote.Position = item.Snippet.Position
			if item.Snippet.ResourceId != nil {
				remote.VideoID = item.Snippet.ResourceId.VideoId
			}
		}
		items = append(items, remote)
	}
	return items, nil
}

func listPlaylistItemsWithRetry(ctx context.Context, service *ytv3.Service, playlistID string, maxResults int64) ([]playlistRemoteItem, error) {
	var lastErr error

	for attempt := 1; attempt <= youtubeSyncListRetryAttempts; attempt++ {
		items, err := listPlaylistItems(ctx, service, playlistID, maxResults)
		if err == nil {
			return items, nil
		}
		lastErr = err

		if !isYouTubePlaylistNotFound(err) || attempt == youtubeSyncListRetryAttempts {
			return nil, err
		}

		delay := time.Duration(attempt) * 250 * time.Millisecond
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, lastErr
}

func insertVideoIntoPlaylist(ctx context.Context, service *ytv3.Service, playlistID, videoID string, position int64) error {
	snippet := &ytv3.PlaylistItemSnippet{
		PlaylistId: playlistID,
		Position:   position,
		ResourceId: &ytv3.ResourceId{
			Kind:    "youtube#video",
			VideoId: videoID,
		},
	}
	// Position uses `omitempty` in the generated API client. Force-send so `0` is
	// transmitted and interpreted as "insert at top".
	snippet.ForceSendFields = append(snippet.ForceSendFields, "Position")

	_, err := service.PlaylistItems.Insert([]string{"snippet"}, &ytv3.PlaylistItem{
		Snippet: snippet,
	}).Context(ctx).Do()
	metrics.ObserveYouTubeAPICall("playlist_sync", "insert_playlist_item", err)
	return err
}

func deletePlaylistItem(ctx context.Context, service *ytv3.Service, playlistItemID string) error {
	err := service.PlaylistItems.Delete(playlistItemID).Context(ctx).Do()
	metrics.ObserveYouTubeAPICall("playlist_sync", "delete_playlist_item", err)
	return err
}

func isYouTubePlaylistNotFound(err error) bool {
	var apiErr *googleapi.Error
	if !stdErrors.As(err, &apiErr) {
		return false
	}
	if apiErr.Code == http.StatusNotFound {
		return true
	}
	for _, item := range apiErr.Errors {
		if item.Reason == "playlistNotFound" {
			return true
		}
	}
	return false
}
