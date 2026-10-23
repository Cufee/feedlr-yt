package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"sync"
	"testing"
)

func TestDesiredMembershipIsAtomicAcrossConnectionsAndDoesNotEvictOnRepeat(t *testing.T) {
	clients := podcastProcessingFixture(t, 3)
	ctx := context.Background()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].SetWatchLaterMembership(ctx, "viewer", "episode-0", true); err != nil {
		t.Fatal(err)
	}
	p, err := clients[0].GetPlaylistBySlug(ctx, "viewer", "watch-later")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = clients[0].db.ExecContext(ctx, "UPDATE playlists SET max_size=2 WHERE id=?", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = clients[0].SetWatchLaterMembership(ctx, "viewer", "episode-1", true); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for n := range 24 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := clients[n%2].SetWatchLaterMembership(ctx, "viewer", "episode-1", true)
			errs <- err
		}(n)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count, err := clients[0].GetPlaylistItemCount(ctx, p.ID); err != nil || count != 2 {
		t.Fatalf("repeated desired state evicted another item: count=%d err=%v", count, err)
	}
	affected, err := clients[0].SetWatchLaterMembership(ctx, "viewer", "episode-2", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(affected) != 2 || affected[0] != "episode-2" {
		t.Fatalf("capacity eviction missing from response: %v", affected)
	}
	if count, err := clients[0].GetPlaylistItemCount(ctx, p.ID); err != nil || count != 2 {
		t.Fatalf("capacity exceeded: %d %v", count, err)
	}
	if _, err := clients[0].SetPlaylistMembership(ctx, "other", p.ID, "episode-2", false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("ownership not enforced: %v", err)
	}
	if _, err := clients[0].SetPlaylistMembership(ctx, "viewer", p.ID, "episode-2", false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("system playlist not protected: %v", err)
	}
}

func TestPatchViewPreservesConcurrentVisibilityAndRemovesCompletedWatchLater(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].db.ExecContext(ctx, "UPDATE videos SET duration=3600 WHERE id='episode-0'"); err != nil {
		t.Fatal(err)
	}
	hidden := true
	if _, err := clients[0].PatchViewState(ctx, "viewer", "episode-0", ViewStatePatch{Hidden: &hidden}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for n := range 12 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			position := 10 + n
			state, err := clients[n%2].PatchViewState(ctx, "viewer", "episode-0", ViewStatePatch{Progress: &position})
			if err == nil && !state.Hidden {
				err = errors.New("progress unhid item")
			}
			errs <- err
		}(n)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := clients[0].SetWatchLaterMembership(ctx, "viewer", "episode-0", true); err != nil {
		t.Fatal(err)
	}
	position := 3300
	state, err := clients[0].PatchViewState(ctx, "viewer", "episode-0", ViewStatePatch{Progress: &position})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Hidden || state.Progress != 3300 {
		t.Fatalf("wrong canonical state: %+v", state)
	}
	p, err := clients[0].GetPlaylistBySlug(ctx, "viewer", "watch-later")
	if err != nil {
		t.Fatal(err)
	}
	if present, err := clients[0].IsVideoInPlaylist(ctx, p.ID, "episode-0"); err != nil || present {
		t.Fatalf("completed item still pinned: %v %v", present, err)
	}
}

func TestSettingsPatchPreservesUnrelatedAndConcurrentFields(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].db.ExecContext(ctx, `INSERT INTO settings(id,user_id,data,created_at,updated_at) VALUES ('preferences','viewer',?,datetime('now'),datetime('now'))`, []byte(`{"PlayerVolume":100,"SponsorBlock":{"SponsorBlockEnabled":true,"SelectedSponsorBlockCategories":["sponsor"]},"PodcastSegments":{"Enabled":true,"SelectedCategories":["intro"]},"FuturePreference":"preserved"}`)); err != nil {
		t.Fatal(err)
	}
	volume := 0
	categories := []string{"selfpromo"}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- clients[0].PatchUserSettings(ctx, "viewer", SettingsPatch{Volume: &volume})
	}()
	go func() {
		defer wg.Done()
		errs <- clients[1].PatchUserSettings(ctx, "viewer", SettingsPatch{SponsorCategories: &categories})
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var gotVolume int
	var category, future string
	if err := clients[0].db.QueryRowContext(ctx, `SELECT json_extract(data,'$.PlayerVolume'),json_extract(data,'$.SponsorBlock.SelectedSponsorBlockCategories[0]'),json_extract(data,'$.FuturePreference') FROM settings WHERE user_id='viewer'`).Scan(&gotVolume, &category, &future); err != nil {
		t.Fatal(err)
	}
	if gotVolume != 0 || category != "selfpromo" || future != "preserved" {
		t.Fatalf("lost partial update: %d %s %s", gotVolume, category, future)
	}
}

func TestConcurrentSettingsInitializationKeepsCanonicalRecordAndEdits(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	defaults := []byte(`{"PlayerVolume":100}`)
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	errs := make(chan error, 16)
	for n := range 16 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			record, err := clients[n%2].GetOrCreateUserSettings(ctx, "viewer", defaults)
			if err != nil {
				errs <- err
				return
			}
			ids <- record.ID
		}(n)
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	canonical := ""
	for id := range ids {
		if canonical == "" {
			canonical = id
		}
		if id != canonical {
			t.Fatal("settings initialization created multiple records")
		}
	}
	volume := 0
	if err := clients[0].PatchUserSettings(ctx, "viewer", SettingsPatch{Volume: &volume}); err != nil {
		t.Fatal(err)
	}
	record, err := clients[1].GetOrCreateUserSettings(ctx, "viewer", defaults)
	if err != nil {
		t.Fatal(err)
	}
	if string(record.Data) != `{"PlayerVolume":0}` {
		t.Fatalf("initialization overwrote existing preference: %s", record.Data)
	}
}

func TestConcurrentPlaylistMovesKeepEveryPositionUnique(t *testing.T) {
	clients := podcastProcessingFixture(t, 4)
	ctx := context.Background()
	if _, err := clients[0].CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	pl := &models.Playlist{UserID: "viewer", Slug: "queue", Name: "Queue"}
	if err := clients[0].CreatePlaylist(ctx, pl); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"episode-0", "episode-1", "episode-2", "episode-3"} {
		if _, err := clients[0].SetPlaylistMembership(ctx, "viewer", pl.ID, id, true); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for n := range 40 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			direction := "up"
			if n%2 == 0 {
				direction = "down"
			}
			id := []string{"episode-0", "episode-1", "episode-2", "episode-3"}[n%4]
			errs <- clients[n%2].SwapPlaylistItemPositions(ctx, pl.ID, id, direction)
		}(n)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := clients[0].GetPlaylistItems(ctx, pl.ID, PlaylistItem.OrderByPosition())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("move changed item count: %d", len(items))
	}
	for position, item := range items {
		if item.Position != int64(position) {
			t.Fatalf("concurrent moves produced duplicate/missing positions: %+v", items)
		}
	}
}
