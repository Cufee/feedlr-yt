package database

import (
	"context"
	"reflect"
	"testing"

	"github.com/cufee/feedlr-yt/internal/database/models"
)

func TestCollectionOrderingKeepsNewestFirstAndStablePages(t *testing.T) {
	c := podcastProcessingFixture(t, 4)[0]
	ctx := context.Background()
	if _, err := c.CreateUser(ctx, "viewer", "viewer"); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ id, sortID, stamp string }{
		{"episode-0", "z", "2024-01-01 00:00:00"},
		{"episode-1", "b", "2024-01-03 00:00:00"},
		{"episode-2", "c", "2024-01-03 00:00:00"},
		{"episode-3", "a", "2024-01-02 00:00:00"},
	} {
		position := 10
		if _, err := c.PatchViewState(ctx, "viewer", fixture.id, ViewStatePatch{Progress: &position}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.db.ExecContext(ctx, "UPDATE views SET id=?,updated_at=? WHERE user_id='viewer' AND video_id=?", "view-"+fixture.sortID, fixture.stamp, fixture.id); err != nil {
			t.Fatal(err)
		}
		if _, err := c.SetWatchLaterMembership(ctx, "viewer", fixture.id, true); err != nil {
			t.Fatal(err)
		}
		if _, err := c.db.ExecContext(ctx, "UPDATE playlist_items SET id=?,created_at=? WHERE video_id=?", "item-"+fixture.sortID, fixture.stamp, fixture.id); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"episode-2", "episode-1", "episode-3", "episode-0"}
	checks := []struct {
		name  string
		query func() ([]*models.View, error)
		want  []string
	}{
		{"all views", func() ([]*models.View, error) { return c.GetUserViews(ctx, "viewer") }, want},
		{"recent limit", func() ([]*models.View, error) { return c.GetRecentUserViews(ctx, "viewer", 2) }, want[:2]},
		{"filtered limit", func() ([]*models.View, error) {
			return c.GetRecentUserViewsForVideoType(ctx, "viewer", "podcast_episode", false, 3)
		}, want[:3]},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			rows, err := check.query()
			if err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for _, row := range rows {
				ids = append(ids, row.VideoID)
			}
			if !reflect.DeepEqual(ids, check.want) {
				t.Fatalf("ordering %v want %v", ids, check.want)
			}
		})
	}
	pl, err := c.GetPlaylistBySlug(ctx, "viewer", "watch-later")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, offset := range []int{0, 2} {
		rows, err := c.GetPlaylistItems(ctx, pl.ID, PlaylistItem.Limit(2), PlaylistItem.Offset(offset))
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			ids = append(ids, row.VideoID)
		}
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("paged saved items %v want %v", ids, want)
	}
}
