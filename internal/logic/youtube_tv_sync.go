package logic

import (
	"context"
	stdErrors "errors"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/types"
	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

const (
	tvSyncUserInactiveDays = 14
	tvSyncMaxUsersPerTick  = 100
)

const (
	tvSyncNoEventTimeout            = 60 * time.Second
	tvSyncReconnectMin              = 10 * time.Second
	tvSyncReconnectMax              = 5 * time.Minute
	tvSyncProgressWriteInterval     = 10 * time.Second
	tvSyncResumeStartWindowSec      = 90
	tvSyncResumeAheadThresholdSec   = 8
	tvSyncMinSkipLengthSec          = 1.0
	tvSyncSkipCooldown              = 1200 * time.Millisecond
	tvSyncStatusUpdateInterval      = 15 * time.Second
	tvSyncWorkerStopTimeout         = 2 * time.Second
	tvSyncConnectionKickTimeout     = 15 * time.Second
	tvSyncNowPlayingPollInterval    = 15 * time.Second
	tvSyncVideoCacheRetryInterval   = 1 * time.Minute
	tvSyncConnectionStateConnectMsg = "Establishing TV session"
	tvSyncNoEventsReason            = "No TV events received"
	tvSyncDeviceName                = "Feedlr TV Sync"
)

const (
	tvSyncStateDisconnected   = "disconnected"
	tvSyncStateConnecting     = "connecting"
	tvSyncStateConnected      = "connected"
	tvSyncStatePausedInactive = "paused_inactive_user"
	tvSyncStatePausedNoEvents = "paused_no_events"
	tvSyncStateDisabled       = "disabled_by_user"
	tvSyncStateError          = "error"
)

var DefaultYouTubeTVSync *YouTubeTVSyncService

type tvSyncWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type tvSyncSegment struct {
	Start float64
	End   float64
}

type tvSyncVideoRuntime struct {
	lastProgressWrite time.Time
	lastVideoCacheTry time.Time
	lastState         string
	sponsorLoaded     bool
	sponsorSegments   []tvSyncSegment
	skippedSegments   map[int]bool
	lastSponsorSkipAt time.Time
	videoCached       bool
}

type tvSyncRuntime struct {
	mu sync.Mutex

	lastEventAt          time.Time
	lastStatusPersistAt  time.Time
	currentVideoID       string
	currentPlaybackState string
	resumeApplied        bool

	sponsorEnabled    bool
	sponsorCategories []sponsorblock.Category
	videoState        map[string]*tvSyncVideoRuntime
}

func newTVSyncRuntime(sponsorEnabled bool, sponsorCategories []sponsorblock.Category) *tvSyncRuntime {
	return &tvSyncRuntime{
		lastEventAt:       time.Now().UTC(),
		sponsorEnabled:    sponsorEnabled,
		sponsorCategories: sponsorCategories,
		videoState:        map[string]*tvSyncVideoRuntime{},
	}
}

func (r *tvSyncRuntime) markEvent(at time.Time) {
	r.mu.Lock()
	r.lastEventAt = at
	r.mu.Unlock()
}

func (r *tvSyncRuntime) eventAge(now time.Time) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastEventAt.IsZero() {
		return time.Hour
	}
	return now.Sub(r.lastEventAt)
}

func (r *tvSyncRuntime) shouldPersistStatus(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastStatusPersistAt.IsZero() {
		r.lastStatusPersistAt = now
		return true
	}
	if now.Sub(r.lastStatusPersistAt) >= tvSyncStatusUpdateInterval {
		r.lastStatusPersistAt = now
		return true
	}
	return false
}

func (r *tvSyncRuntime) setCurrentVideo(videoID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentVideoID == videoID {
		return false
	}
	r.currentVideoID = videoID
	r.currentPlaybackState = ""
	r.resumeApplied = false
	return true
}

func (r *tvSyncRuntime) currentVideo() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentVideoID
}

func (r *tvSyncRuntime) clearCurrentVideo() {
	r.mu.Lock()
	r.currentVideoID = ""
	r.currentPlaybackState = ""
	r.resumeApplied = false
	r.mu.Unlock()
}

func (r *tvSyncRuntime) setCurrentPlaybackState(state string) {
	r.mu.Lock()
	r.currentPlaybackState = state
	r.mu.Unlock()
}

func (r *tvSyncRuntime) currentPlaybackSnapshot() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentVideoID, r.currentPlaybackState
}

func (r *tvSyncRuntime) resumeAppliedForCurrentVideo() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.resumeApplied
}

func (r *tvSyncRuntime) markResumeAppliedForCurrentVideo() {
	r.mu.Lock()
	r.resumeApplied = true
	r.mu.Unlock()
}

func (r *tvSyncRuntime) videoRuntime(videoID string) *tvSyncVideoRuntime {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.videoState[videoID]
	if !ok {
		state = &tvSyncVideoRuntime{
			skippedSegments: map[int]bool{},
		}
		r.videoState[videoID] = state
	}
	return state
}

type YouTubeTVSyncService struct {
	db     youTubeTVSyncStore
	crypto *youtubeSyncCrypto
	lounge *lounge.Client

	noEventTimeout         time.Duration
	watchdogPollInterval   time.Duration
	nowPlayingPollInterval time.Duration
	reconnectMin           time.Duration
	reconnectMax           time.Duration

	metrics *tvSyncMetrics

	workersMu sync.Mutex
	workers   map[string]*tvSyncWorker
}

type youTubeTVSyncStore interface {
	database.YouTubeTVSyncClient
	database.ViewsClient
	database.SettingsClient
}

type watchLaterCleanupDB interface {
	database.PlaylistsClient
	database.VideosClient
}

type tvSyncVideoCacheDB interface {
	database.VideosClient
	database.ChannelsClient
}

type tvSyncMetrics struct {
	connects        atomic.Uint64
	disconnects     atomic.Uint64
	reconnects      atomic.Uint64
	progressUpdates atomic.Uint64
	sponsorSkips    atomic.Uint64
}

func NewYouTubeTVSyncService(db database.Client) (*YouTubeTVSyncService, error) {
	service := &YouTubeTVSyncService{
		db: db,
		crypto: newYouTubeSyncCrypto(
			utils.MustGetEnv("YOUTUBE_SYNC_ENCRYPTION_SECRET"),
		),
		lounge:                 lounge.NewClient(&http.Client{}),
		noEventTimeout:         tvSyncNoEventTimeout,
		watchdogPollInterval:   5 * time.Second,
		nowPlayingPollInterval: tvSyncNowPlayingPollInterval,
		reconnectMin:           tvSyncReconnectMin,
		reconnectMax:           tvSyncReconnectMax,
		metrics:                &tvSyncMetrics{},
		workers:                map[string]*tvSyncWorker{},
	}

	return service, nil
}

func (s *YouTubeTVSyncService) Status(ctx context.Context, userID string) (types.YouTubeTVSyncStatusProps, error) {
	status := types.YouTubeTVSyncStatusProps{
		Available: true,
	}

	account, err := s.db.GetYouTubeTVSyncAccountByUserID(ctx, userID)
	if err != nil {
		if database.IsErrNotFound(err) {
			return status, nil
		}
		return status, err
	}

	status.Enabled = account.SyncEnabled
	status.Connected = account.ConnectionState == tvSyncStateConnected
	status.ConnectionState = account.ConnectionState
	status.StateReason = account.StateReason
	status.ScreenName = account.ScreenName
	status.LastError = account.LastError
	if account.LastConnectedAt.Valid {
		status.LastConnectedAt = account.LastConnectedAt.Time
	}
	if account.LastEventAt.Valid {
		status.LastEventAt = account.LastEventAt.Time
	}
	if account.LastDisconnectAt.Valid {
		status.LastDisconnectAt = account.LastDisconnectAt.Time
	}
	if account.LastUserActivity.Valid {
		status.LastUserActivityAt = account.LastUserActivity.Time
	}
	return status, nil
}

func (s *YouTubeTVSyncService) PairWithCode(ctx context.Context, userID, pairingCode string) error {
	pairingCode = strings.TrimSpace(pairingCode)
	if pairingCode == "" {
		return errors.New("pairing code is required")
	}

	screen, err := s.lounge.PairWithCode(ctx, pairingCode)
	if err != nil {
		return errors.Wrap(err, "failed to pair with tv")
	}

	encrypted, err := s.crypto.Encrypt([]byte(screen.LoungeToken), userID)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt lounge token")
	}

	err = s.db.UpsertYouTubeTVSyncCredentials(ctx, userID, screen.ID, screen.Name, encrypted, s.crypto.secretHash)
	if err != nil {
		return errors.Wrap(err, "failed to persist tv sync credentials")
	}

	s.kickConnectionTick()
	return nil
}

func (s *YouTubeTVSyncService) Disconnect(ctx context.Context, userID string) error {
	s.stopWorker(userID)

	err := s.db.DeleteYouTubeTVSyncAccount(ctx, userID)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	return nil
}

func (s *YouTubeTVSyncService) SetEnabled(ctx context.Context, userID string, enabled bool) error {
	if err := s.db.SetYouTubeTVSyncAccountEnabled(ctx, userID, enabled); err != nil {
		return err
	}

	state := tvSyncStateDisconnected
	if !enabled {
		state = tvSyncStateDisabled
	}

	err := s.db.UpdateYouTubeTVSyncState(ctx, userID, database.YouTubeTVSyncStateUpdate{
		ConnectionState: state,
		StateReason:     "",
		LastError:       "",
	})
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}

	if !enabled {
		s.stopWorker(userID)
	} else {
		s.kickConnectionTick()
	}

	return nil
}

func (s *YouTubeTVSyncService) RunLifecycleTick(ctx context.Context) error {
	accounts, err := s.db.ListEnabledYouTubeTVSyncAccounts(ctx, tvSyncMaxUsersPerTick)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}

	inactiveCutoff := time.Now().UTC().AddDate(0, 0, -tvSyncUserInactiveDays)
	for _, account := range accounts {
		lastActivity, err := s.db.GetUserLastSessionActivity(ctx, account.UserID)
		if err != nil && !database.IsErrNotFound(err) {
			log.Warn().Err(err).Str("userID", account.UserID).Msg("failed to fetch session activity for tv sync")
			continue
		}

		shouldPause := !lastActivity.Valid || lastActivity.Time.Before(inactiveCutoff)
		if shouldPause {
			if account.ConnectionState != tvSyncStatePausedInactive {
				_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
					ConnectionState:  tvSyncStatePausedInactive,
					StateReason:      "No recent login activity",
					LastError:        "",
					LastUserActivity: lastActivity,
				})
			}
			s.stopWorker(account.UserID)
			continue
		}

		if account.ConnectionState == tvSyncStatePausedInactive {
			_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
				ConnectionState:  tvSyncStateDisconnected,
				StateReason:      "",
				LastError:        "",
				LastUserActivity: null.TimeFrom(lastActivity.Time),
			})
			s.kickConnectionTick()
			continue
		}

		if lastActivity.Valid {
			_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
				ConnectionState:  account.ConnectionState,
				StateReason:      account.StateReason,
				LastError:        account.LastError,
				LastUserActivity: lastActivity,
			})
		}
	}

	return nil
}

func (s *YouTubeTVSyncService) RunConnectionTick(ctx context.Context) error {
	accounts, err := s.db.ListEnabledYouTubeTVSyncAccounts(ctx, tvSyncMaxUsersPerTick)
	if err != nil {
		return err
	}

	desired := map[string]*database.YouTubeTVSyncAccount{}
	for _, account := range accounts {
		if !account.SyncEnabled {
			continue
		}
		if account.ConnectionState == tvSyncStatePausedInactive {
			continue
		}
		desired[account.UserID] = account
	}

	var toStop []*tvSyncWorker
	s.workersMu.Lock()
	for userID, worker := range s.workers {
		if _, ok := desired[userID]; ok {
			continue
		}
		toStop = append(toStop, worker)
		delete(s.workers, userID)
	}

	for userID := range desired {
		if _, ok := s.workers[userID]; ok {
			continue
		}

		workerCtx, cancel := context.WithCancel(context.Background())
		worker := &tvSyncWorker{cancel: cancel, done: make(chan struct{})}
		s.workers[userID] = worker

		go s.runWorker(workerCtx, userID, worker)
	}
	s.workersMu.Unlock()

	for _, worker := range toStop {
		worker.cancel()
	}

	return nil
}

func (s *YouTubeTVSyncService) DecryptLoungeToken(account *database.YouTubeTVSyncAccount) ([]byte, error) {
	if account.EncSecretHash != s.crypto.secretHash {
		return nil, errors.New("lounge token secret mismatch")
	}

	decrypted, err := s.crypto.Decrypt(account.LoungeTokenEnc, account.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt lounge token")
	}
	return decrypted, nil
}

func (s *YouTubeTVSyncService) kickConnectionTick() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), tvSyncConnectionKickTimeout)
		defer cancel()

		if err := s.RunConnectionTick(ctx); err != nil {
			log.Warn().Err(err).Msg("failed to run tv sync connection kick")
		}
	}()
}

func (s *YouTubeTVSyncService) stopWorker(userID string) {
	s.workersMu.Lock()
	worker, ok := s.workers[userID]
	if ok {
		delete(s.workers, userID)
	}
	s.workersMu.Unlock()
	if !ok {
		return
	}

	worker.cancel()
	select {
	case <-worker.done:
	case <-time.After(tvSyncWorkerStopTimeout):
	}
}

func (s *YouTubeTVSyncService) runWorker(ctx context.Context, userID string, worker *tvSyncWorker) {
	defer close(worker.done)
	defer s.unregisterWorker(userID, worker)

	backoff := s.reconnectMin
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		account, err := s.db.GetYouTubeTVSyncAccountByUserID(ctx, userID)
		if err != nil {
			if database.IsErrNotFound(err) {
				return
			}
			log.Warn().Err(err).Str("userID", userID).Msg("tv sync failed to load account")
			s.sleepWithContext(ctx, backoff)
			backoff = min(backoff*2, s.reconnectMax)
			continue
		}
		if !account.SyncEnabled || account.ConnectionState == tvSyncStatePausedInactive {
			return
		}

		_ = s.db.UpdateYouTubeTVSyncState(ctx, userID, database.YouTubeTVSyncStateUpdate{
			ConnectionState: tvSyncStateConnecting,
			StateReason:     tvSyncConnectionStateConnectMsg,
			LastError:       "",
		})

		err = s.connectAndRunOnce(ctx, account)
		if stdErrors.Is(err, context.Canceled) || stdErrors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() != nil {
				return
			}
		}

		if err == nil {
			backoff = s.reconnectMin
		} else {
			backoff = min(backoff*2, s.reconnectMax)
			s.recordReconnect(userID, err)
			log.Warn().Err(err).Str("userID", userID).Msg("tv sync worker loop ended")
		}

		s.sleepWithContext(ctx, backoff)
	}
}

func (s *YouTubeTVSyncService) unregisterWorker(userID string, worker *tvSyncWorker) {
	s.workersMu.Lock()
	defer s.workersMu.Unlock()

	current, ok := s.workers[userID]
	if !ok {
		return
	}
	if current == worker {
		delete(s.workers, userID)
	}
}

func (s *YouTubeTVSyncService) sleepWithContext(ctx context.Context, delay time.Duration) {
	if delay <= 0 {
		return
	}
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (s *YouTubeTVSyncService) connectAndRunOnce(ctx context.Context, account *database.YouTubeTVSyncAccount) error {
	session, account, err := s.connectSession(ctx, account)
	if err != nil {
		now := time.Now().UTC()
		_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
			ConnectionState:  tvSyncStateError,
			StateReason:      "Could not connect to TV",
			LastError:        sanitizeError(err),
			LastDisconnectAt: null.TimeFrom(now),
		})
		return err
	}

	sponsorEnabled, sponsorCategories := s.loadSponsorPreferences(ctx, account.UserID)
	runtime := newTVSyncRuntime(sponsorEnabled, sponsorCategories)

	now := time.Now().UTC()
	_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
		ConnectionState: tvSyncStateConnected,
		StateReason:     "",
		LastError:       "",
		LastConnectedAt: null.TimeFrom(now),
		LastEventAt:     null.TimeFrom(now),
	})
	s.recordConnect(account.UserID)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var timedOut atomic.Bool
	watchdogDone := make(chan struct{})
	go func() {
		defer close(watchdogDone)
		pollInterval := s.watchdogPollInterval
		if pollInterval <= 0 {
			pollInterval = 5 * time.Second
		}
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-subCtx.Done():
				return
			case <-ticker.C:
				if runtime.eventAge(time.Now().UTC()) > s.noEventTimeout {
					timedOut.Store(true)
					cancel()
					return
				}
			}
		}
	}()

	nowPlayingPollDone := make(chan struct{})
	go func() {
		defer close(nowPlayingPollDone)
		pollInterval := s.nowPlayingPollInterval
		if pollInterval <= 0 {
			return
		}

		requestNowPlaying := func() {
			err := s.lounge.GetNowPlaying(subCtx, session)
			if err == nil {
				return
			}
			if subCtx.Err() != nil {
				return
			}

			if stdErrors.Is(err, lounge.ErrAuthExpired) || stdErrors.Is(err, lounge.ErrUnknownSID) || stdErrors.Is(err, lounge.ErrSessionGone) {
				log.Warn().Err(err).Str("userID", account.UserID).Msg("tv sync nowPlaying poll failed with session error")
				cancel()
				return
			}

			log.Debug().Err(err).Str("userID", account.UserID).Msg("tv sync nowPlaying poll failed")
		}

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-subCtx.Done():
				return
			case <-ticker.C:
				videoID, playbackState := runtime.currentPlaybackSnapshot()
				if videoID == "" || playbackState != "1" {
					continue
				}
				requestNowPlaying()
			}
		}
	}()

	subscribeErr := s.lounge.Subscribe(subCtx, session, func(event lounge.Event) error {
		now := time.Now().UTC()
		runtime.markEvent(now)

		if runtime.shouldPersistStatus(now) {
			update := database.YouTubeTVSyncStateUpdate{
				ConnectionState: tvSyncStateConnected,
				StateReason:     "",
				LastError:       "",
				LastEventAt:     null.TimeFrom(now),
			}
			if videoID := runtime.currentVideo(); videoID != "" {
				update.LastVideoID = null.StringFrom(videoID)
			}
			_ = s.db.UpdateYouTubeTVSyncState(subCtx, account.UserID, update)
		}

		return s.processEvent(subCtx, account, session, runtime, event)
	})

	cancel()
	<-watchdogDone
	<-nowPlayingPollDone

	if ctx.Err() != nil {
		return ctx.Err()
	}

	now = time.Now().UTC()
	if timedOut.Load() {
		_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
			ConnectionState:  tvSyncStatePausedNoEvents,
			StateReason:      tvSyncNoEventsReason,
			LastError:        "",
			LastDisconnectAt: null.TimeFrom(now),
		})
		s.recordDisconnect(account.UserID, tvSyncStatePausedNoEvents, nil)
		return errors.New("tv sync subscription paused: no events")
	}

	if subscribeErr != nil {
		if stdErrors.Is(subscribeErr, context.Canceled) && ctx.Err() != nil {
			return ctx.Err()
		}

		_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
			ConnectionState:  tvSyncStateDisconnected,
			StateReason:      "Session ended",
			LastError:        sanitizeError(subscribeErr),
			LastDisconnectAt: null.TimeFrom(now),
		})
		s.recordDisconnect(account.UserID, tvSyncStateDisconnected, subscribeErr)
		return subscribeErr
	}

	_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
		ConnectionState:  tvSyncStateDisconnected,
		StateReason:      "Session ended",
		LastError:        "",
		LastDisconnectAt: null.TimeFrom(now),
	})
	s.recordDisconnect(account.UserID, tvSyncStateDisconnected, nil)
	return nil
}

func (s *YouTubeTVSyncService) connectSession(ctx context.Context, account *database.YouTubeTVSyncAccount) (*lounge.Session, *database.YouTubeTVSyncAccount, error) {
	token, err := s.DecryptLoungeToken(account)
	if err != nil {
		return nil, account, err
	}

	session, err := s.lounge.Connect(ctx, account.ScreenID, string(token), tvSyncDeviceName)
	if err == nil {
		return session, account, nil
	}
	if !stdErrors.Is(err, lounge.ErrAuthExpired) {
		return nil, account, err
	}

	refreshed, refreshErr := s.lounge.RefreshLoungeToken(ctx, account.ScreenID)
	if refreshErr != nil {
		return nil, account, errors.Wrap(refreshErr, "failed to refresh lounge token")
	}
	encrypted, encErr := s.crypto.Encrypt([]byte(refreshed.LoungeToken), account.UserID)
	if encErr != nil {
		return nil, account, errors.Wrap(encErr, "failed to encrypt refreshed lounge token")
	}

	if refreshed.Name == "" {
		refreshed.Name = account.ScreenName
	}
	if err := s.db.UpdateYouTubeTVSyncLoungeToken(ctx, account.UserID, refreshed.ID, refreshed.Name, encrypted, s.crypto.secretHash); err != nil {
		return nil, account, errors.Wrap(err, "failed to persist refreshed lounge token")
	}

	account, err = s.db.GetYouTubeTVSyncAccountByUserID(ctx, account.UserID)
	if err != nil {
		return nil, account, err
	}
	token, err = s.DecryptLoungeToken(account)
	if err != nil {
		return nil, account, err
	}

	session, err = s.lounge.Connect(ctx, account.ScreenID, string(token), tvSyncDeviceName)
	if err != nil {
		return nil, account, err
	}
	return session, account, nil
}

func (s *YouTubeTVSyncService) processEvent(ctx context.Context, account *database.YouTubeTVSyncAccount, session *lounge.Session, runtime *tvSyncRuntime, event lounge.Event) error {
	if event.Type == "loungeScreenDisconnected" {
		return errors.New("screen disconnected")
	}

	if event.Type == "onPlaybackSpeedChanged" {
		if err := s.lounge.GetNowPlaying(ctx, session); err != nil {
			log.Debug().Err(err).Str("userID", account.UserID).Msg("failed to request nowPlaying after playback speed change")
		}
	}

	playback, ok := lounge.ExtractPlaybackEvent(event)
	if !ok {
		return nil
	}

	videoID := strings.TrimSpace(playback.VideoID)
	if videoID == "" {
		videoID = runtime.currentVideo()
	}
	if videoID == "" {
		return nil
	}

	isNewVideo := runtime.setCurrentVideo(videoID)
	if isNewVideo {
		_ = s.db.UpdateYouTubeTVSyncState(ctx, account.UserID, database.YouTubeTVSyncStateUpdate{
			ConnectionState: tvSyncStateConnected,
			StateReason:     "",
			LastError:       "",
			LastVideoID:     null.StringFrom(videoID),
		})
		if !playback.HasCurrentTime {
			if err := s.lounge.GetNowPlaying(ctx, session); err != nil {
				log.Debug().Err(err).Str("userID", account.UserID).Str("videoID", videoID).Msg("failed to request nowPlaying for new video")
			}
		}
	}

	videoRuntime := runtime.videoRuntime(videoID)
	now := time.Now().UTC()
	state := strings.TrimSpace(playback.State)
	if state != "" {
		runtime.setCurrentPlaybackState(state)
	}
	if !runtime.resumeAppliedForCurrentVideo() && shouldAttemptResumeSeek(isNewVideo, playback, state, videoRuntime.lastState) {
		saved := s.getStoredProgress(ctx, account.UserID, videoID)
		if shouldApplyResumeSeek(saved, playback) {
			if err := s.lounge.SeekTo(ctx, session, float64(saved)); err != nil {
				log.Debug().Err(err).Str("userID", account.UserID).Str("videoID", videoID).Int("saved_progress", saved).Msg("failed to apply tv resume seek")
			} else {
				videoRuntime.lastSponsorSkipAt = now
				log.Debug().Str("userID", account.UserID).Str("videoID", videoID).Int("saved_progress", saved).Msg("applied tv resume seek from app state")
			}
			runtime.markResumeAppliedForCurrentVideo()
			if state != "" {
				videoRuntime.lastState = state
			}
			// Never persist pre-seek TV timestamps from the same event.
			return nil
		}
		runtime.markResumeAppliedForCurrentVideo()
	}

	observedSecond := 0
	if playback.HasCurrentTime {
		observedSecond = clampPlaybackSecond(playback.CurrentTime, playback.Duration, playback.HasDuration)
	}

	if playback.HasCurrentTime {
		if runtime.sponsorEnabled {
			s.processSponsorSkip(ctx, account.UserID, videoID, videoRuntime, playback, session, now, runtime.sponsorCategories)
		}

		progressWriteAllowed := true
		if cacheDB, ok := s.db.(tvSyncVideoCacheDB); ok {
			progressWriteAllowed = s.ensureVideoCachedForProgress(ctx, cacheDB, account.UserID, videoID, videoRuntime, now)
		}

		if progressWriteAllowed && shouldWriteProgress(playback.State, now, videoRuntime.lastProgressWrite) {
			resolved, err := UpdateViewProgress(ctx, s.db, account.UserID, videoID, observedSecond)
			if err != nil {
				log.Warn().Err(err).Str("userID", account.UserID).Str("videoID", videoID).Msg("failed to sync tv progress")
			} else {
				s.recordProgressUpdate(account.UserID, videoID, observedSecond, resolved)
				videoRuntime.lastProgressWrite = now
				if cleanupDB, ok := s.db.(watchLaterCleanupDB); ok {
					_ = RemoveFromWatchLaterIfFullyWatched(ctx, cleanupDB, account.UserID, videoID, resolved)
				}
			}
		}
	}
	if state != "" {
		videoRuntime.lastState = state
	}
	if state == "0" {
		runtime.clearCurrentVideo()
	}
	return nil
}

func shouldWriteProgress(state string, now time.Time, lastWrite time.Time) bool {
	if state != "" && state != "1" && state != "2" && state != "0" {
		return false
	}
	if lastWrite.IsZero() {
		return true
	}
	return now.Sub(lastWrite) >= tvSyncProgressWriteInterval
}

func shouldAttemptResumeSeek(isNewVideo bool, playback lounge.PlaybackEvent, state, lastState string) bool {
	if !playback.HasCurrentTime {
		return false
	}
	currentSecond := clampPlaybackSecond(playback.CurrentTime, playback.Duration, playback.HasDuration)
	if isNewVideo && currentSecond <= tvSyncResumeStartWindowSec {
		return true
	}
	return state == "1" && lastState != "" && lastState != "1"
}

func shouldApplyResumeSeek(savedProgress int, playback lounge.PlaybackEvent) bool {
	if savedProgress <= 0 || !playback.HasCurrentTime {
		return false
	}

	currentSecond := clampPlaybackSecond(playback.CurrentTime, playback.Duration, playback.HasDuration)
	if currentSecond <= tvSyncResumeStartWindowSec {
		return true
	}

	return savedProgress >= currentSecond+tvSyncResumeAheadThresholdSec
}

func clampPlaybackSecond(current float64, duration float64, hasDuration bool) int {
	seconds := int(math.Floor(current))
	if seconds < 0 {
		seconds = 0
	}
	if hasDuration {
		d := int(math.Floor(duration))
		if d > 0 && seconds > d {
			seconds = d
		}
	}
	return seconds
}

func (s *YouTubeTVSyncService) getStoredProgress(ctx context.Context, userID, videoID string) int {
	views, err := s.db.GetUserViews(ctx, userID, videoID)
	if err != nil || len(views) == 0 {
		return 0
	}
	for _, view := range views {
		if view.VideoID == videoID {
			return int(view.Progress)
		}
	}
	return 0
}

func (s *YouTubeTVSyncService) ensureVideoCachedForProgress(ctx context.Context, cacheDB tvSyncVideoCacheDB, userID, videoID string, state *tvSyncVideoRuntime, now time.Time) bool {
	if state.videoCached {
		return true
	}
	if !state.lastVideoCacheTry.IsZero() && now.Sub(state.lastVideoCacheTry) < tvSyncVideoCacheRetryInterval {
		return false
	}
	state.lastVideoCacheTry = now

	if err := EnsureVideoCached(ctx, cacheDB, videoID); err != nil {
		log.Warn().Err(err).Str("userID", userID).Str("videoID", videoID).Msg("failed to cache tv video before progress sync")
		return false
	}

	state.videoCached = true
	return true
}

func (s *YouTubeTVSyncService) processSponsorSkip(ctx context.Context, userID, videoID string, state *tvSyncVideoRuntime, playback lounge.PlaybackEvent, session *lounge.Session, now time.Time, categories []sponsorblock.Category) {
	if !state.sponsorLoaded {
		state.sponsorLoaded = true
		state.sponsorSegments = s.loadSponsorSegments(videoID, categories)
	}
	if len(state.sponsorSegments) == 0 {
		return
	}
	if now.Sub(state.lastSponsorSkipAt) < tvSyncSkipCooldown {
		return
	}
	if !playback.HasCurrentTime {
		return
	}

	for idx, segment := range state.sponsorSegments {
		if state.skippedSegments[idx] {
			continue
		}
		if playback.CurrentTime < segment.Start || playback.CurrentTime >= segment.End {
			continue
		}

		if err := s.lounge.SeekTo(ctx, session, segment.End); err != nil {
			log.Debug().Err(err).Str("userID", userID).Str("videoID", videoID).Msg("failed to skip sponsor segment")
			return
		}

		state.skippedSegments[idx] = true
		state.lastSponsorSkipAt = now
		s.recordSponsorSkip(userID, videoID, segment.Start, segment.End)
		return
	}
}

func (s *YouTubeTVSyncService) loadSponsorSegments(videoID string, categories []sponsorblock.Category) []tvSyncSegment {
	if len(categories) == 0 {
		return nil
	}

	raw, err := sponsorblock.C.GetVideoSegments(videoID, categories...)
	if err != nil {
		return nil
	}
	return normalizeSponsorSegments(raw)
}

func normalizeSponsorSegments(segments []sponsorblock.Segment) []tvSyncSegment {
	normalized := make([]tvSyncSegment, 0, len(segments))
	for _, segment := range segments {
		if len(segment.Segment) != 2 {
			continue
		}
		start := segment.Segment[0]
		end := segment.Segment[1]
		if end <= start {
			continue
		}
		if end-start < tvSyncMinSkipLengthSec {
			continue
		}
		normalized = append(normalized, tvSyncSegment{Start: start, End: end})
	}
	if len(normalized) == 0 {
		return nil
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Start < normalized[j].Start
	})

	merged := make([]tvSyncSegment, 0, len(normalized))
	for _, segment := range normalized {
		if len(merged) == 0 {
			merged = append(merged, segment)
			continue
		}
		last := &merged[len(merged)-1]
		if segment.Start <= last.End {
			if segment.End > last.End {
				last.End = segment.End
			}
			continue
		}
		merged = append(merged, segment)
	}

	return merged
}

func (s *YouTubeTVSyncService) loadSponsorPreferences(ctx context.Context, userID string) (bool, []sponsorblock.Category) {
	settings, err := GetUserSettings(ctx, s.db, userID)
	if err != nil {
		log.Debug().Err(err).Str("userID", userID).Msg("failed to load settings for sponsor preferences")
		return false, nil
	}
	if !settings.SponsorBlock.SponsorBlockEnabled {
		return false, nil
	}

	categories := make([]sponsorblock.Category, 0, len(settings.SponsorBlock.SelectedSponsorBlockCategories))
	for _, value := range settings.SponsorBlock.SelectedSponsorBlockCategories {
		idx := slices.IndexFunc(sponsorblock.AvailableCategories, func(category sponsorblock.Category) bool {
			return category.Value == value
		})
		if idx < 0 {
			continue
		}
		categories = append(categories, sponsorblock.AvailableCategories[idx])
	}

	return true, categories
}

func (s *YouTubeTVSyncService) recordConnect(userID string) {
	if s.metrics == nil {
		return
	}
	total := s.metrics.connects.Add(1)
	log.Info().
		Str("userID", userID).
		Uint64("connect_total", total).
		Msg("tv sync connected")
}

func (s *YouTubeTVSyncService) recordDisconnect(userID, state string, err error) {
	if s.metrics == nil {
		return
	}
	total := s.metrics.disconnects.Add(1)
	entry := log.Info().
		Str("userID", userID).
		Str("state", state).
		Uint64("disconnect_total", total)
	if err != nil {
		entry = entry.Str("reason", sanitizeError(err))
	}
	entry.Msg("tv sync disconnected")
}

func (s *YouTubeTVSyncService) recordReconnect(userID string, err error) {
	if s.metrics == nil {
		return
	}
	total := s.metrics.reconnects.Add(1)
	log.Info().
		Str("userID", userID).
		Str("reason", sanitizeError(err)).
		Uint64("reconnect_total", total).
		Msg("tv sync reconnect scheduled")
}

func (s *YouTubeTVSyncService) recordProgressUpdate(userID, videoID string, incoming, resolved int) {
	if s.metrics == nil {
		return
	}
	total := s.metrics.progressUpdates.Add(1)
	log.Debug().
		Str("userID", userID).
		Str("videoID", videoID).
		Int("incoming_progress", incoming).
		Int("resolved_progress", resolved).
		Uint64("progress_updates_total", total).
		Msg("tv sync progress updated")
}

func (s *YouTubeTVSyncService) recordSponsorSkip(userID, videoID string, start, end float64) {
	if s.metrics == nil {
		return
	}
	total := s.metrics.sponsorSkips.Add(1)
	log.Debug().
		Str("userID", userID).
		Str("videoID", videoID).
		Float64("segment_start", start).
		Float64("segment_end", end).
		Uint64("sponsor_skips_total", total).
		Msg("tv sync sponsor segment skipped")
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
