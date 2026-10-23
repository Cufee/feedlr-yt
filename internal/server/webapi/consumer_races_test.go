package webapi

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
)

type pausedSettingsRead struct {
	database.Client
	pending, release chan struct{}
	once             sync.Once
}

func (d *pausedSettingsRead) GetUserSettings(ctx context.Context, userID string) (*models.Setting, error) {
	record, err := d.Client.GetUserSettings(ctx, userID)
	d.once.Do(func() { close(d.pending); <-d.release })
	return record, err
}

func awaitPaused(t *testing.T, pending <-chan struct{}) {
	t.Helper()
	select {
	case <-pending:
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not reach the controlled interleaving")
	}
}
func awaitFinished(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not finish")
	}
}

func TestLegacySettingsHydrationCannotRevertConcurrentJSONPreference(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// This record predates podcast settings and triggers read-time defaults.
	if err := f.db.UpsertSettings(ctx, &models.Setting{UserID: "viewer", Data: []byte(`{"PlayerVolume":100,"SponsorBlock":{"SponsorBlockEnabled":true,"SelectedSponsorBlockCategories":["sponsor"],"AvailableSponsorBlockCategories":[]}}`)}); err != nil {
		t.Fatal(err)
	}
	d := &pausedSettingsRead{Client: f.db, pending: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(d.release) }) }
	defer release()
	done := make(chan error, 1)
	go func() { _, err := logic.GetUserSettings(ctx, d, "viewer"); done <- err }()
	awaitPaused(t, d.pending)
	res, v := f.request(t, "PATCH", "/settings", `{"sponsorBlock":{"enabled":false},"podcastSegments":{"enabled":false}}`, f.session, "")
	release()
	awaitFinished(t, done)
	if res.StatusCode != 204 {
		t.Fatalf("preference update: %d %#v", res.StatusCode, v)
	}
	record, err := f.db.GetUserSettings(ctx, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	var persisted struct {
		SponsorBlock    struct{ SponsorBlockEnabled bool }
		PodcastSegments struct{ Enabled bool }
	}
	if err = json.Unmarshal(record.Data, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.SponsorBlock.SponsorBlockEnabled || persisted.PodcastSegments.Enabled {
		t.Fatal("a stale settings read replaced a successful preference update")
	}
	settings, err := logic.GetUserSettings(ctx, f.db, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	if settings.PodcastSegments.Enabled || len(settings.PodcastSegments.AvailableCategories) == 0 {
		t.Fatal("hydration did not preserve an explicit disabled preference and supply available categories")
	}
}

type pausedTVState struct {
	database.Client
	pending, release chan struct{}
	once             sync.Once
}

func (d *pausedTVState) pause() { d.once.Do(func() { close(d.pending); <-d.release }) }
func (d *pausedTVState) GetUserViews(ctx context.Context, userID string, ids ...string) ([]*models.View, error) {
	records, err := d.Client.GetUserViews(ctx, userID, ids...)
	d.pause()
	return records, err
}
func (d *pausedTVState) PatchViewState(ctx context.Context, userID, mediaID string, patch database.ViewStatePatch) (database.ViewState, error) {
	d.pause()
	return d.Client.PatchViewState(ctx, userID, mediaID, patch)
}

func TestTVProgressCannotClearConcurrentJSONVisibilityIntent(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	position := 20
	if _, err := f.db.PatchViewState(ctx, "viewer", "pe_first", database.ViewStatePatch{Progress: &position}); err != nil {
		t.Fatal(err)
	}
	d := &pausedTVState{Client: f.db, pending: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(d.release) }) }
	defer release()
	done := make(chan error, 1)
	go func() { _, err := logic.UpdateViewProgress(ctx, d, "viewer", "pe_first", 21); done <- err }()
	awaitPaused(t, d.pending)
	res, v := f.request(t, "PATCH", "/media/pe_first/state", `{"hidden":true}`, f.session, "")
	release()
	awaitFinished(t, done)
	if res.StatusCode != 204 {
		t.Fatalf("visibility update: %d %#v", res.StatusCode, v)
	}
	views, err := f.db.GetUserViews(ctx, "viewer", "pe_first")
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || !views[0].Hidden.Bool || views[0].Progress != 21 {
		t.Fatalf("TV progress replaced a visibility intent: %#v", views)
	}
}

func TestLegacyToggleCannotReplaceAnotherConcurrentJSONPreference(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	res, v := f.request(t, "GET", "/settings", "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("initial settings: %d %#v", res.StatusCode, v)
	}
	d := &pausedSettingsRead{Client: f.db, pending: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(d.release) }) }
	defer release()
	done := make(chan error, 1)
	go func() { _, err := logic.TogglePodcastSegments(ctx, d, "viewer"); done <- err }()
	awaitPaused(t, d.pending)
	res, v = f.request(t, "PATCH", "/settings", `{"sponsorBlock":{"enabled":false}}`, f.session, "")
	release()
	awaitFinished(t, done)
	if res.StatusCode != 204 {
		t.Fatalf("preference update: %d %#v", res.StatusCode, v)
	}
	settings, err := logic.GetUserSettings(ctx, f.db, "viewer")
	if err != nil {
		t.Fatal(err)
	}
	if settings.SponsorBlock.SponsorBlockEnabled || settings.PodcastSegments.Enabled {
		t.Fatal("legacy toggle replaced an unrelated preference changed by the JSON client")
	}
}
