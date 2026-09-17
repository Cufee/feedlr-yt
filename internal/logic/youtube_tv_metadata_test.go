package logic

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/api/youtube/lounge"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

type tvMetadataTestStore struct {
	*mockTVSyncStore
	database.VideosClient
	database.ChannelsClient
	metadataMu sync.Mutex
	videos     map[string]*models.Video
	channels   map[string]*models.Channel
	views      map[string]*models.View
}

func newTVMetadataTestStore() *tvMetadataTestStore {
	return &tvMetadataTestStore{mockTVSyncStore: &mockTVSyncStore{}, videos: make(map[string]*models.Video), channels: make(map[string]*models.Channel), views: make(map[string]*models.View)}
}

func (s *tvMetadataTestStore) GetVideoByID(_ context.Context, id string, _ ...database.VideoQuery) (*models.Video, error) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if v := s.videos[id]; v != nil {
		copy := *v
		return &copy, nil
	}
	return nil, sql.ErrNoRows
}

func (s *tvMetadataTestStore) UpsertVideos(_ context.Context, videos ...*models.Video) error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	for _, v := range videos {
		if s.channels[v.ChannelID] == nil {
			return errors.New("FOREIGN KEY constraint failed")
		}
		copy := *v
		s.videos[v.ID] = &copy
	}
	return nil
}

func (s *tvMetadataTestStore) GetChannel(_ context.Context, id string, _ ...database.ChannelQuery) (*models.Channel, error) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if c := s.channels[id]; c != nil {
		copy := *c
		return &copy, nil
	}
	return nil, sql.ErrNoRows
}

func (s *tvMetadataTestStore) UpsertChannel(_ context.Context, channel *models.Channel) error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	copy := *channel
	s.channels[channel.ID] = &copy
	return nil
}

func (s *tvMetadataTestStore) GetUserViews(_ context.Context, user string, ids ...string) ([]*models.View, error) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	var views []*models.View
	for _, id := range ids {
		if v := s.views[user+":"+id]; v != nil {
			copy := *v
			views = append(views, &copy)
		}
	}
	return views, nil
}

func (s *tvMetadataTestStore) UpsertView(_ context.Context, view *models.View) error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if s.videos[view.VideoID] == nil {
		return errors.New("FOREIGN KEY constraint failed")
	}
	copy := *view
	s.views[view.UserID+":"+view.VideoID] = &copy
	return nil
}

func testTVVideo(id string) *youtube.VideoDetails {
	return &youtube.VideoDetails{Video: youtube.Video{ID: id, Title: "Video title", Type: youtube.VideoTypeVideo}, ChannelID: "UCYO_jab_esuFRV4b17AJtAw", ChannelTitle: "Channel name", Duration: 300}
}

func waitForTVImports(t *testing.T, m *tvMetadataImporter) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		pending := len(m.jobs)
		m.mu.Unlock()
		if pending == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("TV imports did not finish")
}

func TestTVMetadataImportPreservesLatestProgressAndFinalEvent(t *testing.T) {
	store := newTVMetadataTestStore()
	s := &YouTubeTVSyncService{db: store}
	m := newTVMetadataImporter(store, s.persistTVProgress)
	s.metadata = m
	started, release := make(chan struct{}), make(chan struct{})
	var videoCalls atomic.Int32
	m.fetchVideo = func(ctx context.Context, id string) (*youtube.VideoDetails, error) {
		videoCalls.Add(1)
		close(started)
		select {
		case <-release:
			return testTVVideo(id), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	m.fetchChannel = func(_ context.Context, id string) (*youtube.Channel, error) {
		return &youtube.Channel{ID: id, Title: "Public channel", Thumbnail: "https://example.com/avatar", Description: "About the channel"}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runtime := newTVSyncRuntime(false, nil)
	account := &database.YouTubeTVSyncAccount{UserID: "user"}
	event := func(second int, state string) {
		t.Helper()
		if err := s.processEvent(ctx, account, nil, runtime, lounge.Event{Type: "nowPlaying", Args: []any{map[string]any{"videoId": "video", "currentTime": second, "duration": 300, "state": state}}}); err != nil {
			t.Fatal(err)
		}
	}
	event(120, "1")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("import did not start")
	}
	event(30, "1") // A backward seek inside the ten-second write interval.
	m.queue(ctx, "second-user", "video", 80, false)
	event(35, "0") // Also inside the interval; must not be dropped.
	if runtime.currentVideo() != "" {
		t.Fatal("ended unknown video was not cleared")
	}
	cancel() // The final observation survives the lounge connection closing.
	close(release)
	waitForTVImports(t, m)
	if videoCalls.Load() != 1 {
		t.Fatalf("video fetched %d times", videoCalls.Load())
	}
	for user, want := range map[string]int64{"user": 35, "second-user": 80} {
		views, _ := store.GetUserViews(context.Background(), user, "video")
		if len(views) != 1 || views[0].Progress != want {
			t.Fatalf("%s: got %+v, want %d", user, views, want)
		}
	}
	channel, _ := store.GetChannel(context.Background(), testTVVideo("video").ChannelID)
	if channel.Title != "Public channel" || channel.Thumbnail == "" {
		t.Fatalf("channel metadata missing: %+v", channel)
	}
}

func TestTVMetadataChannelFailureFallsBackAndBacksOff(t *testing.T) {
	store := newTVMetadataTestStore()
	m := newTVMetadataImporter(store, nil)
	var channelCalls int
	m.fetchVideo = func(_ context.Context, id string) (*youtube.VideoDetails, error) { return testTVVideo(id), nil }
	m.fetchChannel = func(context.Context, string) (*youtube.Channel, error) {
		channelCalls++
		return nil, errors.New("HTTP 429")
	}
	for _, id := range []string{"first", "second"} {
		if err := m.ensureVideo(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if channelCalls != 1 {
		t.Fatalf("channel fetched %d times during cooldown", channelCalls)
	}
	channel, _ := store.GetChannel(context.Background(), testTVVideo("first").ChannelID)
	if channel.Title != "Channel name" {
		t.Fatalf("missing fallback channel: %+v", channel)
	}
	channel.UploadsPlaylistID = "existing-playlist"
	channel.FeedUpdatedAt = time.Now().UTC()
	if err := store.UpsertChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.retryAfter["channel:"+channel.ID] = time.Time{}
	m.mu.Unlock()
	m.fetchChannel = func(_ context.Context, id string) (*youtube.Channel, error) {
		return &youtube.Channel{ID: id, Title: "Enriched", Thumbnail: "avatar"}, nil
	}
	if err := m.ensureVideo(context.Background(), "third"); err != nil {
		t.Fatal(err)
	}
	enriched, _ := store.GetChannel(context.Background(), channel.ID)
	if enriched.Title != "Enriched" || enriched.UploadsPlaylistID != channel.UploadsPlaylistID || !enriched.FeedUpdatedAt.Equal(channel.FeedUpdatedAt) {
		t.Fatalf("channel state lost: %+v", enriched)
	}
}

func TestTVMetadataVideoRetriesAndCooldown(t *testing.T) {
	store := newTVMetadataTestStore()
	m := newTVMetadataImporter(store, func(context.Context, string, string, int) error { t.Error("persisted failed import"); return nil })
	m.retryDelays = []time.Duration{0, 0}
	var calls atomic.Int32
	m.fetchVideo = func(context.Context, string) (*youtube.VideoDetails, error) {
		calls.Add(1)
		return nil, errors.New("player unavailable")
	}
	m.queue(context.Background(), "user", "video", 20, false)
	waitForTVImports(t, m)
	if calls.Load() != 3 {
		t.Fatalf("got %d attempts", calls.Load())
	}
	if m.queue(context.Background(), "user", "video", 30, false) {
		t.Fatal("queued import during failure cooldown")
	}
	if len(store.videos) != 0 || len(store.channels) != 0 {
		t.Fatal("failed import cached metadata")
	}
}

func TestTVMetadataReusesCachedVideoAndChannel(t *testing.T) {
	store := newTVMetadataTestStore()
	m := newTVMetadataImporter(store, nil)
	channel := &models.Channel{ID: testTVVideo("video").ChannelID, Title: "Cached", Thumbnail: "avatar"}
	if err := store.UpsertChannel(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	var calls int
	m.fetchVideo = func(_ context.Context, id string) (*youtube.VideoDetails, error) {
		calls++
		return testTVVideo(id), nil
	}
	m.fetchChannel = func(context.Context, string) (*youtube.Channel, error) {
		t.Fatal("fetched cached channel")
		return nil, nil
	}
	for range 2 {
		if err := m.ensureVideo(context.Background(), "video"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetched cached video: %d calls", calls)
	}
}

func TestTVMetadataSharesChannelFetchAcrossVideos(t *testing.T) {
	store := newTVMetadataTestStore()
	m := newTVMetadataImporter(store, nil)
	m.fetchVideo = func(_ context.Context, id string) (*youtube.VideoDetails, error) { return testTVVideo(id), nil }
	var calls atomic.Int32
	m.fetchChannel = func(_ context.Context, id string) (*youtube.Channel, error) {
		calls.Add(1)
		return &youtube.Channel{ID: id, Title: "Shared", Thumbnail: "avatar"}, nil
	}
	var wg sync.WaitGroup
	for _, id := range []string{"first", "second", "third"} {
		wg.Go(func() {
			if err := m.ensureVideo(context.Background(), id); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("channel fetched %d times", calls.Load())
	}
}

func TestTVMetadataRetriesPendingProgressWrite(t *testing.T) {
	store := newTVMetadataTestStore()
	s := &YouTubeTVSyncService{db: store}
	var attempts atomic.Int32
	m := newTVMetadataImporter(store, func(ctx context.Context, userID, videoID string, second int) error {
		if attempts.Add(1) == 1 {
			return errors.New("database is locked")
		}
		return s.persistTVProgress(ctx, userID, videoID, second)
	})
	m.retryDelays = []time.Duration{0}
	m.fetchVideo = func(_ context.Context, id string) (*youtube.VideoDetails, error) { return testTVVideo(id), nil }
	m.fetchChannel = func(context.Context, string) (*youtube.Channel, error) { return nil, errors.New("unavailable") }
	m.queue(context.Background(), "user", "video", 42, false)
	waitForTVImports(t, m)
	views, _ := store.GetUserViews(context.Background(), "user", "video")
	if attempts.Load() != 2 || len(views) != 1 || views[0].Progress != 42 {
		t.Fatalf("pending progress was lost: attempts=%d views=%+v", attempts.Load(), views)
	}
}
