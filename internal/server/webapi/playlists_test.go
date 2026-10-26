package webapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
)

func TestAPIPlaylistProgressUsesWatchTime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		durations []int64
		progress  []int64
		want      int
	}{
		{name: "partial_playback_is_weighted_by_duration", durations: []int64{120, 1080}, progress: []int64{30, 270}, want: 25},
		{name: "finishing_short_video_does_not_count_half_the_playlist", durations: []int64{120, 1080}, progress: []int64{120, 0}, want: 10},
		{name: "completion_buffer_does_not_inflate_watch_time", durations: []int64{120}, progress: []int64{100}, want: 83},
		{name: "progress_cannot_exceed_video_duration", durations: []int64{120, 1080}, progress: []int64{1000, 0}, want: 10},
		{name: "unknown_duration_is_excluded_from_both_totals", durations: []int64{0, 120}, progress: []int64{1000, 30}, want: 25},
		{name: "all_durations_unknown", durations: []int64{0, 0}, progress: []int64{1000, 30}},
		{name: "untouched_playlist", durations: []int64{120, 1080}, progress: []int64{0, 0}},
		{name: "empty_playlist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			playlist, err := logic.CreateUserPlaylist(ctx, f.db, "viewer", "Watch time", "")
			if err != nil {
				t.Fatal(err)
			}
			for i, duration := range tc.durations {
				id := []string{"pe_first", "pe_second"}[i]
				video, err := f.db.GetVideoByID(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				video.Duration = duration
				if err := f.db.UpsertVideos(ctx, video); err != nil {
					t.Fatal(err)
				}
				if err := f.db.AddPlaylistItem(ctx, playlist.ID, id); err != nil {
					t.Fatal(err)
				}
				if tc.progress[i] > 0 {
					if err := f.db.UpsertView(ctx, &models.View{UserID: "viewer", VideoID: id, Progress: tc.progress[i]}); err != nil {
						t.Fatal(err)
					}
				}
				// Another viewer's completed playback must not affect this playlist.
				if err := f.db.UpsertView(ctx, &models.View{UserID: "other", VideoID: id, Progress: 100000}); err != nil {
					t.Fatal(err)
				}
			}
			assertPlaylistProgress(t, f, playlist.ID, tc.want)
		})
	}
}

func TestAPIPlaylistProgressHonorsManualCompletionAndReset(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	playlist, err := logic.CreateUserPlaylist(ctx, f.db, "viewer", "Watch time", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pe_first", "pe_second"} {
		if err := f.db.AddPlaylistItem(ctx, playlist.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct {
		body string
		want int
	}{
		{body: `{"completed":true}`, want: 50},
		{body: `{"completed":false}`, want: 0},
	} {
		res, value := f.request(t, http.MethodPatch, "/media/pe_first/state", step.body, f.session, "")
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("updating completion: %d %#v", res.StatusCode, value)
		}
		assertPlaylistProgress(t, f, playlist.ID, step.want)
	}
}

func assertPlaylistProgress(t *testing.T, f *fixture, playlistID string, want int) {
	t.Helper()
	res, value := f.request(t, http.MethodGet, "/playlists", "", f.session, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("playlists: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "PlaylistList")
	items := value.(map[string]any)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one playlist, got %d", len(items))
	}
	if got := items[0].(map[string]any)["progressPercent"]; got != float64(want) {
		t.Errorf("playlist list progress: got %v, want %d", got, want)
	}
	res, value = f.request(t, http.MethodGet, "/playlists/"+playlistID, "", f.session, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("playlist detail: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "PlaylistDetail")
	if got := value.(map[string]any)["playlist"].(map[string]any)["progressPercent"]; got != float64(want) {
		t.Errorf("playlist detail progress: got %v, want %d", got, want)
	}
}
