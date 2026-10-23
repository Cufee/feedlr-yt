package webapi

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
)

func TestAPIFeedRoundsVisibleSectionsToResponsiveGridRows(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		new, watched, hidden int
		wantNew, wantWatched int
	}{
		{name: "split_after_hidden_items", new: 13, watched: 23, hidden: 3, wantNew: 12, wantWatched: 12},
		{name: "small_watched_section", new: 31, watched: 5, wantNew: 24, wantWatched: 5},
		{name: "small_new_section", new: 5, watched: 31, wantNew: 5, wantWatched: 24},
		{name: "small_feed", new: 1, watched: 1, wantNew: 1, wantWatched: 1},
		{name: "complete_batches", new: 12, watched: 12, wantNew: 12, wantWatched: 12},
		{name: "new_items_keep_existing_priority_and_cap", new: 36, watched: 3, wantNew: 36},
		{name: "watched_only", watched: 36, wantWatched: 36},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			published := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)
			var newIDs, watchedIDs []string
			for i := range tc.new + tc.watched + tc.hidden {
				id := fmt.Sprintf("feed-%03d", i)
				if i < 2 {
					id = []string{"pe_first", "pe_second"}[i]
				}
				if err := f.db.UpsertVideos(ctx, &models.Video{
					ID: id, ChannelID: "show", Title: id, Type: "podcast_episode", Duration: 3600,
					MediaURL:    null.StringFrom("https://example.test/" + id + ".mp3"),
					PublishedAt: published.Add(-time.Duration(i) * time.Minute),
				}); err != nil {
					t.Fatal(err)
				}
				hidden := i < tc.hidden
				var progress int64
				switch {
				case hidden:
				case i < tc.hidden+tc.new:
					newIDs = append(newIDs, id)
					if len(newIDs) == 1 {
						progress = 120 // Incomplete playback remains in the new section.
					}
				default:
					watchedIDs = append(watchedIDs, id)
					progress = 3600
				}
				if hidden || progress > 0 {
					if err := f.db.UpsertView(ctx, &models.View{UserID: "viewer", VideoID: id, Progress: progress, Hidden: null.BoolFrom(hidden)}); err != nil {
						t.Fatal(err)
					}
				}
			}

			// YouTube sync shares this logic; grid rounding belongs to the HTTP response only.
			unrounded, err := logic.GetUserVideosProps(ctx, f.db, "viewer")
			if err != nil {
				t.Fatal(err)
			}
			logicNew := min(tc.new, 36)
			logicWatched := min(tc.watched, 36-logicNew)
			if len(unrounded.New) != logicNew || len(unrounded.Watched) != logicWatched {
				t.Fatalf("shared feed changed: got %d new and %d watched, want %d and %d", len(unrounded.New), len(unrounded.Watched), logicNew, logicWatched)
			}

			res, value := f.request(t, "GET", "/feed", "", f.session, "")
			if res.StatusCode != http.StatusOK {
				t.Fatalf("feed: %d %#v", res.StatusCode, value)
			}
			f.check(t, value, "Feed")
			feed := value.(map[string]any)
			if feed["onboarding"] != false {
				t.Fatalf("nonempty feed was treated as onboarding: %#v", feed)
			}
			for _, section := range []struct {
				name      string
				ids       []string
				completed bool
			}{
				{name: "new", ids: newIDs[:tc.wantNew]},
				{name: "watched", ids: watchedIDs[:tc.wantWatched], completed: true},
			} {
				items := feed[section.name].([]any)
				var gotIDs []string
				for _, raw := range items {
					item := raw.(map[string]any)
					gotIDs = append(gotIDs, item["id"].(string))
					if item["hidden"] != false || item["completed"] != section.completed {
						t.Fatalf("%s contains an incorrectly classified item: %#v", section.name, item)
					}
				}
				if !slices.Equal(gotIDs, section.ids) {
					t.Fatalf("%s items or publication order: got %v, want %v", section.name, gotIDs, section.ids)
				}
			}
		})
	}
}
