package logic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

type tvMetadataStore interface {
	database.VideosClient
	database.ChannelsClient
}

type tvMetadataJob struct {
	// Keep the latest observation, including backward seeks, for each user.
	progress map[string]int
}

// TV imports deliberately bypass GetVideoByID and CacheChannel: those helpers
// can call Data API v3. Jobs also keep network requests off the lounge event loop.
type tvMetadataImporter struct {
	db              tvMetadataStore
	fetchVideo      func(context.Context, string) (*youtube.VideoDetails, error)
	fetchChannel    func(context.Context, string) (*youtube.Channel, error)
	persistProgress func(context.Context, string, string, int) error

	mu          sync.Mutex
	jobs        map[string]*tvMetadataJob
	retryAfter  map[string]time.Time
	channels    singleflight.Group
	slots       chan struct{}
	retryDelays []time.Duration
}

func newTVMetadataImporter(db tvMetadataStore, persist func(context.Context, string, string, int) error) *tvMetadataImporter {
	return &tvMetadataImporter{
		db: db, persistProgress: persist,
		fetchVideo: func(ctx context.Context, id string) (*youtube.VideoDetails, error) {
			if youtube.DefaultClient == nil {
				return nil, fmt.Errorf("YouTube player client is unavailable")
			}
			return youtube.DefaultClient.GetVideoPlayerDetailsContext(ctx, id)
		},
		fetchChannel: func(ctx context.Context, id string) (*youtube.Channel, error) {
			if youtube.DefaultClient == nil {
				return nil, fmt.Errorf("YouTube player client is unavailable")
			}
			return youtube.DefaultClient.GetChannelPage(ctx, id)
		},
		jobs: make(map[string]*tvMetadataJob), retryAfter: make(map[string]time.Time),
		slots: make(chan struct{}, 3), retryDelays: []time.Duration{2 * time.Second, 10 * time.Second},
	}
}

// queue updates an existing import before any direct progress write can overtake
// it. Set existingOnly=false after a missing-video foreign-key failure.
func (m *tvMetadataImporter) queue(ctx context.Context, userID, videoID string, second int, existingOnly bool) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[videoID]; job != nil {
		job.progress[userID] = second
		return true
	}
	if existingOnly || ctx.Err() != nil || len(m.jobs) >= 128 || time.Now().Before(m.retryAfter["video:"+videoID]) {
		return false
	}
	job := &tvMetadataJob{progress: map[string]int{userID: second}}
	m.jobs[videoID] = job
	// A final playback event must survive the lounge session disconnecting.
	// Imports are bounded in time and count; they are not a durable job queue.
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 90*time.Second)
	go func() {
		defer cancel()
		m.run(jobCtx, videoID, job)
	}()
	return true
}

func (m *tvMetadataImporter) run(ctx context.Context, videoID string, job *tvMetadataJob) {
	var err error
	defer func() {
		if err != nil {
			m.cooldown("video:" + videoID)
			m.mu.Lock()
			delete(m.jobs, videoID)
			m.mu.Unlock()
			log.Warn().Err(err).Str("videoID", videoID).Msg("failed to import TV video and pending progress")
		}
	}()
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		err = ctx.Err()
		return
	}
	for attempt := 0; ; attempt++ {
		err = m.ensureVideo(ctx, videoID)
		if err == nil {
			break
		}
		if attempt >= len(m.retryDelays) || ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(m.retryDelays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			err = ctx.Err()
			return
		case <-timer.C:
		}
	}
	for {
		m.mu.Lock()
		pending := job.progress
		if len(pending) == 0 {
			delete(m.jobs, videoID)
			m.mu.Unlock()
			return
		}
		job.progress = make(map[string]int)
		m.mu.Unlock()
		for userID, second := range pending {
			if writeErr := m.writePendingProgress(ctx, userID, videoID, second); writeErr != nil {
				log.Warn().Err(writeErr).Str("userID", userID).Str("videoID", videoID).Msg("failed to save imported TV video progress")
			}
		}
	}
}

// Retry transient database failures without losing a final observation.
func (m *tvMetadataImporter) writePendingProgress(ctx context.Context, userID, videoID string, second int) error {
	for attempt := 0; ; attempt++ {
		err := m.persistProgress(ctx, userID, videoID, second)
		if err == nil || attempt >= len(m.retryDelays) {
			return err
		}
		timer := time.NewTimer(m.retryDelays[attempt])
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (m *tvMetadataImporter) ensureVideo(ctx context.Context, videoID string) error {
	video, err := m.db.GetVideoByID(ctx, videoID)
	if err != nil && !database.IsErrNotFound(err) {
		return err
	}
	if video != nil {
		return nil
	}
	details, err := m.fetchVideo(ctx, videoID)
	if err != nil {
		return err
	}
	if details == nil || details.ID != videoID || strings.TrimSpace(details.ChannelID) == "" || strings.TrimSpace(details.Title) == "" || details.Type == youtube.VideoTypeFailed || details.Type == youtube.VideoTypePrivate {
		return fmt.Errorf("player returned unavailable or incomplete video metadata")
	}
	if err := m.ensureChannel(ctx, details.ChannelID, details.ChannelTitle); err != nil {
		return err
	}
	return UpdateVideoCache(ctx, m.db, details)
}

func (m *tvMetadataImporter) ensureChannel(ctx context.Context, channelID, title string) error {
	_, err, _ := m.channels.Do(channelID, func() (any, error) {
		existing, err := m.db.GetChannel(ctx, channelID)
		if err != nil && !database.IsErrNotFound(err) {
			return nil, err
		}
		if existing != nil && existing.Title != "" && existing.Thumbnail != "" {
			return nil, nil
		}
		m.mu.Lock()
		coolingDown := time.Now().Before(m.retryAfter["channel:"+channelID])
		m.mu.Unlock()
		var details *youtube.Channel
		if !coolingDown {
			details, err = m.fetchChannel(ctx, channelID)
			if err == nil && (details == nil || details.ID != channelID || strings.TrimSpace(details.Title) == "") {
				err = fmt.Errorf("channel page returned incomplete metadata")
			}
			if err != nil {
				details = nil
				m.cooldown("channel:" + channelID)
				log.Debug().Err(err).Str("channelID", channelID).Msg("using player channel metadata after channel page failure")
			}
		}
		// Re-read after the network request to preserve any richer metadata and
		// feed state another importer may have saved in the meantime.
		existing, err = m.db.GetChannel(ctx, channelID)
		if err != nil && !database.IsErrNotFound(err) {
			return nil, err
		}
		if existing == nil {
			existing = &models.Channel{ID: channelID, Title: strings.TrimSpace(title)}
			if existing.Title == "" {
				existing.Title = channelID
			}
		}
		if details != nil {
			existing.Title = details.Title
			if details.Description != "" {
				existing.Description = details.Description
			}
			if details.Thumbnail != "" {
				existing.Thumbnail = details.Thumbnail
			}
		}
		return nil, m.db.UpsertChannel(ctx, existing)
	})
	return err
}

func (m *tvMetadataImporter) cooldown(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for key, until := range m.retryAfter {
		if !now.Before(until) {
			delete(m.retryAfter, key)
		}
	}
	m.retryAfter[key] = now.Add(time.Minute)
}

func (s *YouTubeTVSyncService) persistTVProgress(ctx context.Context, userID, videoID string, second int) error {
	resolved, err := UpdateViewProgress(ctx, s.db, userID, videoID, second)
	if err != nil {
		return err
	}
	s.recordProgressUpdate(userID, videoID, second, resolved)
	if cleanupDB, ok := s.db.(watchLaterCleanupDB); ok {
		_ = RemoveFromWatchLaterIfFullyWatched(ctx, cleanupDB, userID, videoID, resolved)
	}
	return nil
}
