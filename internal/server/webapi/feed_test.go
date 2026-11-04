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

func TestAPIFeedFillsCombinedLimitWithUnwatchedFirst(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		new, watched, hidden int
		wantNew, wantWatched int
	}{
		{name: "split_after_hidden_items", new: 13, watched: 23, hidden: 3, wantNew: 13, wantWatched: 11},
		{name: "more_unwatched_than_limit", new: 31, watched: 5, wantNew: 24},
		{name: "watched_fills_remaining_slots", new: 5, watched: 31, wantNew: 5, wantWatched: 19},
		{name: "small_feed", new: 1, watched: 1, wantNew: 1, wantWatched: 1},
		{name: "incomplete_grid_row", new: 4, wantNew: 4},
		{name: "incomplete_sections", new: 14, watched: 5, wantNew: 14, wantWatched: 5},
		{name: "complete_batches", new: 12, watched: 12, wantNew: 12, wantWatched: 12},
		{name: "new_items_keep_priority_and_cap", new: 36, watched: 3, wantNew: 24},
		{name: "watched_only", watched: 36, wantWatched: 24},
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
				default:
					watchedIDs = append(watchedIDs, id)
					progress = 3600
					if len(watchedIDs) == 1 {
						progress = 120 // Partially played items follow untouched items.
					}
				}
				// A reset view record must behave like an untouched item.
				if hidden || progress > 0 || len(newIDs) == 1 {
					if err := f.db.UpsertView(ctx, &models.View{UserID: "viewer", VideoID: id, Progress: progress, Hidden: null.BoolFrom(hidden)}); err != nil {
						t.Fatal(err)
					}
				}
			}

			// YouTube sync retains its larger window; the home API limits the combined feed.
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
				name string
				ids  []string
			}{
				{name: "new", ids: newIDs[:tc.wantNew]},
				{name: "watched", ids: watchedIDs[:tc.wantWatched]},
			} {
				items := feed[section.name].([]any)
				var gotIDs []string
				for _, raw := range items {
					item := raw.(map[string]any)
					gotIDs = append(gotIDs, item["id"].(string))
					completed := section.name == "watched" && len(gotIDs) > 1
					if item["hidden"] != false || item["completed"] != completed {
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

func TestAPIFeedKeepsOlderUnwatchedItemsAcrossRefreshAndPlayback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	published := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)
	var ids []string
	for i := range 40 {
		id := fmt.Sprintf("feed-%03d", i)
		if i < 2 {
			id = []string{"pe_first", "pe_second"}[i]
		}
		ids = append(ids, id)
		if err := f.db.UpsertVideos(ctx, &models.Video{
			ID: id, ChannelID: "show", Title: id, Type: "podcast_episode", Duration: 3600,
			MediaURL:    null.StringFrom("https://example.test/" + id + ".mp3"),
			PublishedAt: published.Add(-time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if i < 39 {
			if err := f.db.UpsertView(ctx, &models.View{UserID: "viewer", VideoID: id, Progress: 120}); err != nil {
				t.Fatal(err)
			}
		}
	}

	assertFeed := func(wantNew, wantWatched []string) {
		t.Helper()
		for range 3 {
			res, value := f.request(t, http.MethodGet, "/feed", "", f.session, "")
			if res.StatusCode != http.StatusOK {
				t.Fatalf("feed: %d %#v", res.StatusCode, value)
			}
			feed := value.(map[string]any)
			if len(feed["new"].([]any))+len(feed["watched"].([]any)) != 24 {
				t.Fatalf("home feed did not fill 24 cards: %#v", feed)
			}
			for _, section := range []struct {
				name string
				ids  []string
			}{{"new", wantNew}, {"watched", wantWatched}} {
				var got []string
				for _, raw := range feed[section.name].([]any) {
					got = append(got, raw.(map[string]any)["id"].(string))
				}
				if !slices.Equal(got, section.ids) {
					t.Fatalf("%s after refresh: got %v, want %v", section.name, got, section.ids)
				}
			}
		}
	}
	// The oldest item still takes priority over newer partially played items.
	assertFeed(ids[39:], ids[:23])
	for _, progress := range []int{120, 3600, 0} {
		res, value := f.request(t, http.MethodPatch, "/media/"+ids[39]+"/state", fmt.Sprintf(`{"progressSeconds":%d}`, progress), f.session, "")
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("playback update: %d %#v", res.StatusCode, value)
		}
		if progress == 0 {
			assertFeed(ids[39:], ids[:23])
		} else {
			// Once played, only the tail is dropped to keep the grid at 24 cards.
			assertFeed(nil, ids[:24])
		}
	}
}
