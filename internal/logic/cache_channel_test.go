package logic

import (
	"context"
	"database/sql"
	stdErrors "errors"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

const cacheTestChannelID = "UCYO_jab_esuFRV4b17AJtAw"

type cacheChannelStore struct {
	database.ChannelsClient
	channel   *models.Channel
	getErr    error
	upsertErr error
	upserts   []*models.Channel
}

func (s *cacheChannelStore) GetChannel(context.Context, string, ...database.ChannelQuery) (*models.Channel, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.channel == nil {
		return nil, sql.ErrNoRows
	}
	result := *s.channel
	return &result, nil
}

func (s *cacheChannelStore) UpsertChannel(_ context.Context, channel *models.Channel) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	copy := *channel
	s.channel = &copy
	s.upserts = append(s.upserts, &copy)
	return nil
}

func TestCacheChannelBackfillsDerivedPlaylistWithoutFetchingFreshMetadata(t *testing.T) {
	store := &cacheChannelStore{channel: &models.Channel{
		ID:        cacheTestChannelID,
		Title:     "Cached title",
		UpdatedAt: time.Now(),
	}}
	fetchCalls := 0

	channel, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(context.Context, string) (*youtube.Channel, error) {
		fetchCalls++
		return nil, stdErrors.New("fresh channel should not be fetched")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cached {
		t.Fatal("expected cached channel")
	}
	if fetchCalls != 0 {
		t.Fatalf("metadata fetched %d times", fetchCalls)
	}
	if channel.UploadsPlaylistID != "UUYO_jab_esuFRV4b17AJtAw" {
		t.Fatalf("unexpected uploads playlist %q", channel.UploadsPlaylistID)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("derived playlist was persisted %d times", len(store.upserts))
	}
}

func TestCacheChannelRefreshesFromPublicMetadataAndPreservesLocalState(t *testing.T) {
	feedUpdatedAt := time.Now().Add(-3 * time.Hour).UTC()
	store := &cacheChannelStore{channel: &models.Channel{
		ID:                cacheTestChannelID,
		Title:             "Old title",
		Description:       "Existing description",
		Thumbnail:         "existing-avatar",
		FeedUpdatedAt:     feedUpdatedAt,
		UploadsPlaylistID: "legacy-value",
		UpdatedAt:         time.Now().Add(-8 * 24 * time.Hour),
	}}

	channel, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(_ context.Context, id string) (*youtube.Channel, error) {
		return &youtube.Channel{ID: id, Title: "Public title"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cached {
		t.Fatal("expected refresh of an existing channel to remain cached")
	}
	if channel.Title != "Public title" || channel.Description != "Existing description" || channel.Thumbnail != "existing-avatar" {
		t.Fatalf("metadata was not merged safely: %+v", channel)
	}
	if !channel.FeedUpdatedAt.Equal(feedUpdatedAt) {
		t.Fatalf("feed timestamp changed from %v to %v", feedUpdatedAt, channel.FeedUpdatedAt)
	}
	if channel.UploadsPlaylistID != "UUYO_jab_esuFRV4b17AJtAw" {
		t.Fatalf("unexpected uploads playlist %q", channel.UploadsPlaylistID)
	}
}

func TestCacheChannelUsesStaleRecordWhenPublicFetchFails(t *testing.T) {
	store := &cacheChannelStore{channel: &models.Channel{
		ID:          cacheTestChannelID,
		Title:       "Stale but usable",
		Description: "Keep me",
		UpdatedAt:   time.Now().Add(-8 * 24 * time.Hour),
	}}

	channel, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(context.Context, string) (*youtube.Channel, error) {
		return nil, stdErrors.New("public page throttled")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cached || channel.Title != "Stale but usable" || channel.Description != "Keep me" {
		t.Fatalf("stale channel was not preserved: cached=%v channel=%+v", cached, channel)
	}
	if channel.UploadsPlaylistID != "UUYO_jab_esuFRV4b17AJtAw" {
		t.Fatalf("stale channel missing derived playlist: %+v", channel)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("failed refresh changed the stale record %d times", len(store.upserts))
	}
}

func TestCacheChannelStoresNewPublicChannelWithDerivedPlaylist(t *testing.T) {
	store := &cacheChannelStore{}
	channel, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(_ context.Context, id string) (*youtube.Channel, error) {
		return &youtube.Channel{
			ID:          id,
			Title:       "Public title",
			Description: "Public description",
			Thumbnail:   "public-avatar",
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if cached {
		t.Fatal("new channel reported as cached")
	}
	if channel.Title != "Public title" || channel.Description != "Public description" || channel.Thumbnail != "public-avatar" {
		t.Fatalf("public metadata was not stored: %+v", channel)
	}
	if channel.UploadsPlaylistID != "UUYO_jab_esuFRV4b17AJtAw" {
		t.Fatalf("unexpected uploads playlist %q", channel.UploadsPlaylistID)
	}
	if len(store.upserts) != 1 {
		t.Fatalf("new channel persisted %d times", len(store.upserts))
	}
}

func TestCacheChannelFallsBackOnIncompletePublicMetadata(t *testing.T) {
	store := &cacheChannelStore{channel: &models.Channel{
		ID:        cacheTestChannelID,
		Title:     "Stale title",
		UpdatedAt: time.Now().Add(-8 * 24 * time.Hour),
	}}
	channel, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(context.Context, string) (*youtube.Channel, error) {
		return &youtube.Channel{ID: cacheTestChannelID}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cached || channel.Title != "Stale title" {
		t.Fatalf("incomplete public metadata replaced stale channel: %+v", channel)
	}
}

func TestCacheChannelRequiresPublicMetadataForUncachedChannel(t *testing.T) {
	store := &cacheChannelStore{}
	_, cached, err := cacheChannel(context.Background(), store, cacheTestChannelID, func(context.Context, string) (*youtube.Channel, error) {
		return nil, stdErrors.New("public page unavailable")
	})
	if err == nil {
		t.Fatal("expected uncached channel fetch to fail")
	}
	if cached {
		t.Fatal("uncached channel reported as cached")
	}
	if len(store.upserts) != 0 {
		t.Fatal("failed channel fetch was persisted")
	}
}

func TestCacheChannelDoesNotHideCancellationBehindStaleData(t *testing.T) {
	store := &cacheChannelStore{channel: &models.Channel{
		ID:        cacheTestChannelID,
		Title:     "Stale",
		UpdatedAt: time.Now().Add(-8 * 24 * time.Hour),
	}}
	ctx, cancel := context.WithCancel(context.Background())

	_, _, err := cacheChannel(ctx, store, cacheTestChannelID, func(ctx context.Context, _ string) (*youtube.Channel, error) {
		cancel()
		return nil, ctx.Err()
	})
	if !stdErrors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context cancellation", err)
	}
	if len(store.upserts) != 0 {
		t.Fatal("canceled refresh persisted stale data")
	}
}

func TestCacheChannelRejectsInvalidIDBeforeDatabaseOrNetwork(t *testing.T) {
	store := &cacheChannelStore{getErr: stdErrors.New("database should not be called")}
	fetched := false
	_, _, err := cacheChannel(context.Background(), store, "not-a-channel", func(context.Context, string) (*youtube.Channel, error) {
		fetched = true
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected invalid channel ID error")
	}
	if fetched {
		t.Fatal("invalid channel ID reached metadata fetch")
	}
}
