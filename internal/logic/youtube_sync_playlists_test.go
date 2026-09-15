package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"google.golang.org/api/option"
	ytv3 "google.golang.org/api/youtube/v3"
)

type playlistSyncDB struct {
	database.Client
	playlists  []*models.Playlist
	watchLater *models.Playlist
	items      map[string][]*models.PlaylistItem
	states     map[string]*models.YoutubeSyncTarget
	feedID     string
}

func (db *playlistSyncDB) GetUserPlaylists(context.Context, string) ([]*models.Playlist, error) {
	return db.playlists, nil
}

func (db *playlistSyncDB) GetPlaylistBySlug(context.Context, string, string) (*models.Playlist, error) {
	if db.watchLater == nil {
		return nil, sql.ErrNoRows
	}
	return db.watchLater, nil
}

func (db *playlistSyncDB) GetPlaylistItems(_ context.Context, id string, _ ...database.PlaylistItemQuery) ([]*models.PlaylistItem, error) {
	return db.items[id], nil
}

func (db *playlistSyncDB) ListYouTubeSyncTargets(context.Context, string) ([]*models.YoutubeSyncTarget, error) {
	var result []*models.YoutubeSyncTarget
	for _, state := range db.states {
		copy := *state
		result = append(result, &copy)
	}
	return result, nil
}

func (db *playlistSyncDB) UpsertYouTubeSyncTarget(_ context.Context, state *models.YoutubeSyncTarget) error {
	copy := *state
	db.states[state.SourceID] = &copy
	return nil
}

func (db *playlistSyncDB) UpdateYouTubeSyncPlaylistID(_ context.Context, _, id string) error {
	db.feedID = id
	return nil
}

func syncTestItem(id, kind string) *models.PlaylistItem {
	item := &models.PlaylistItem{VideoID: id}
	item.R = item.R.NewStruct()
	item.R.Video = &models.Video{ID: id, Type: kind}
	return item
}

type playlistSyncAPI struct {
	t          *testing.T
	playlists  map[string][]*ytv3.PlaylistItem
	metadata   map[string]*ytv3.Playlist
	writes     int
	serial     int
	pages      int
	failInsert bool
	failPage   bool
}

func (api *playlistSyncAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/youtube/v3")
	if r.Method != http.MethodGet {
		api.writes++
	}
	if r.URL.Path == "/playlists" {
		var p ytv3.Playlist
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			api.t.Error(err)
		}
		if r.Method == http.MethodPost {
			api.serial++
			p.Id = fmt.Sprintf("export-%d", api.serial)
			api.playlists[p.Id] = nil
			if p.Status == nil || p.Status.PrivacyStatus != "private" {
				api.t.Error("export must be private")
			}
		}
		api.metadata[p.Id] = &p
		json.NewEncoder(w).Encode(p)
		return
	}
	if r.URL.Path != "/playlistItems" {
		api.t.Errorf("unexpected path %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("playlistId")
		items, ok := api.playlists[id]
		if !ok {
			http.Error(w, `{"error":{"code":404,"message":"missing","errors":[{"reason":"playlistNotFound"}]}}`, 404)
			return
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("pageToken"))
		if start > 0 && api.failPage {
			http.Error(w, `{"error":{"code":500}}`, 500)
			return
		}
		end := min(start+50, len(items))
		result := ytv3.PlaylistItemListResponse{Items: items[start:end]}
		if end < len(items) {
			result.NextPageToken = strconv.Itoa(end)
		}
		api.pages++
		json.NewEncoder(w).Encode(result)
		return
	}
	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		for playlist, items := range api.playlists {
			for i, item := range items {
				if item.Id == id {
					api.playlists[playlist] = slices.Delete(items, i, i+1)
					api.positions(playlist)
					w.WriteHeader(204)
					return
				}
			}
		}
		api.t.Errorf("unknown deleted item %s", id)
		return
	}
	var item ytv3.PlaylistItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		api.t.Error(err)
	}
	if api.failInsert && r.Method == http.MethodPost {
		http.Error(w, `{"error":{"code":403,"message":"failed insert"}}`, 403)
		return
	}
	id := item.Snippet.PlaylistId
	items := api.playlists[id]
	if r.Method == http.MethodPut {
		i := slices.IndexFunc(items, func(v *ytv3.PlaylistItem) bool { return v.Id == item.Id })
		if i < 0 {
			api.t.Error("unknown moved item")
			return
		}
		items = slices.Delete(items, i, i+1)
	} else {
		api.serial++
		item.Id = fmt.Sprintf("item-%d", api.serial)
	}
	position := int(item.Snippet.Position)
	if position > len(items) {
		api.t.Errorf("invalid position %d in %d items", position, len(items))
		http.Error(w, `{"error":{"code":400}}`, 400)
		return
	}
	api.playlists[id] = slices.Insert(items, position, &item)
	api.positions(id)
	json.NewEncoder(w).Encode(item)
}

func (api *playlistSyncAPI) positions(id string) {
	for i, item := range api.playlists[id] {
		item.Snippet.Position = int64(i)
	}
}

func newPlaylistSyncAPI(t *testing.T) (*playlistSyncAPI, *ytv3.Service) {
	t.Helper()
	api := &playlistSyncAPI{t: t, playlists: make(map[string][]*ytv3.PlaylistItem), metadata: make(map[string]*ytv3.Playlist)}
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	service, err := ytv3.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return api, service
}

func TestSyncAllPlaylistsConvergesWithinSharedBudget(t *testing.T) {
	api, service := newPlaylistSyncAPI(t)
	api.playlists["existing-feed"] = nil
	db := &playlistSyncDB{
		playlists:  []*models.Playlist{{ID: "local", Name: "Learning"}, {ID: "import", Name: "Imported", YoutubePlaylistID: null.StringFrom("original")}},
		watchLater: &models.Playlist{ID: "later", Name: "Watch Later", Slug: WatchLaterSlug, System: true},
		items:      map[string][]*models.PlaylistItem{"later": {syncTestItem("later-video", "video")}, "import": {syncTestItem("import-video", "video")}},
		states:     make(map[string]*models.YoutubeSyncTarget),
	}
	var desired []string
	for i := range 65 {
		id := fmt.Sprintf("video-%02d", i)
		desired = append(desired, id)
		db.items["local"] = append(db.items["local"], syncTestItem(id, "video"))
	}
	db.items["local"] = append(db.items["local"], syncTestItem("podcast", "podcast_episode"))
	s := &YouTubeSyncService{db: db, maxExpensiveCallsPerSync: 4}
	account := &models.YoutubeSyncAccount{ID: "account", UserID: "user", PlaylistID: null.StringFrom("existing-feed")}
	for range 30 {
		before := api.writes
		if err := s.syncPlaylists(context.Background(), service, account, []string{"feed-video"}); err != nil {
			t.Fatal(err)
		}
		if api.writes-before > 4 {
			t.Fatal("per-user budget exceeded")
		}
	}
	assertVideos := func(source string, want []string) {
		t.Helper()
		state := db.states[source]
		if state == nil {
			t.Fatalf("no destination for %s", source)
		}
		var got []string
		for _, item := range api.playlists[state.PlaylistID] {
			got = append(got, item.Snippet.ResourceId.VideoId)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v, want %v", source, got, want)
		}
	}
	assertVideos("feed", []string{"feed-video"})
	assertVideos("playlist:local", desired)
	assertVideos("playlist:later", []string{"later-video"})
	assertVideos("playlist:import", []string{"import-video"})
	if db.states["feed"].PlaylistID != "existing-feed" {
		t.Fatal("existing feed was replaced")
	}
	if _, exists := api.playlists["original"]; exists {
		t.Fatal("import source was used as an export")
	}
	if len(api.metadata) != 3 {
		t.Fatalf("expected three new playlists, got %d", len(api.metadata))
	}
	before := api.writes
	if err := s.syncPlaylists(context.Background(), service, account, []string{"feed-video"}); err != nil {
		t.Fatal(err)
	}
	if api.writes != before {
		t.Fatal("unchanged playlists caused writes")
	}

	// Reordering an existing list and renaming it must converge without duplicates.
	slices.Reverse(db.items["local"])
	slices.Reverse(desired)
	db.playlists[0].Name = "Renamed"
	for range 20 {
		if err := s.syncPlaylists(context.Background(), service, account, []string{"feed-video"}); err != nil {
			t.Fatal(err)
		}
	}
	assertVideos("playlist:local", desired)
	if api.metadata[db.states["playlist:local"].PlaylistID].Snippet.Title != "Feedlr: Renamed" {
		t.Fatal("rename was not synced")
	}
	// Emptying a playlist removes all pages of remote items over multiple runs.
	db.items["local"] = nil
	for range 20 {
		if err := s.syncPlaylists(context.Background(), service, account, nil); err != nil {
			t.Fatal(err)
		}
	}
	assertVideos("playlist:local", nil)
	assertVideos("feed", nil)
}

func TestSyncPlaylistFailuresAndRecovery(t *testing.T) {
	api, service := newPlaylistSyncAPI(t)
	db := &playlistSyncDB{states: make(map[string]*models.YoutubeSyncTarget)}
	s := &YouTubeSyncService{db: db}
	account := &models.YoutubeSyncAccount{ID: "account", UserID: "user"}
	state := &models.YoutubeSyncTarget{AccountID: "account", SourceID: "feed", PlaylistID: "deleted"}
	source := youtubePlaylistSource{state: state, title: youtubeSyncPlaylistName, description: youtubeSyncPlaylistDescription, feed: []string{"a", "b", "c"}}
	budget := 1
	if err := s.syncPlaylistSource(context.Background(), service, account, source, &budget); err != nil {
		t.Fatal(err)
	}
	if budget != 0 || state.PlaylistID == "deleted" || db.feedID != state.PlaylistID {
		t.Fatal("deleted destination was not recreated and persisted within budget")
	}
	api.failInsert = true
	budget = 4
	before := api.writes
	if err := s.syncPlaylistSource(context.Background(), service, account, source, &budget); err == nil {
		t.Fatal("expected insert error")
	}
	if api.writes-before != 1 || budget != 3 {
		t.Fatal("must stop after failed insert and count the attempted write")
	}
}

func TestListPlaylistItemsDiscardsPartialPagesOnError(t *testing.T) {
	api, service := newPlaylistSyncAPI(t)
	for i := range 55 {
		api.playlists["large"] = append(api.playlists["large"], &ytv3.PlaylistItem{Id: strconv.Itoa(i)})
	}
	api.failPage = true
	items, err := listPlaylistItems(context.Background(), service, "large", 50)
	if err == nil || len(items) != 0 {
		t.Fatal("must not reconcile against incomplete remote contents")
	}
}

func TestPlaylistSourcesFairnessAndIsolation(t *testing.T) {
	db := &playlistSyncDB{
		playlists: []*models.Playlist{{ID: "new", Name: "New"}},
		states:    map[string]*models.YoutubeSyncTarget{"feed": {AccountID: "account", SourceID: "feed", LastAttemptAt: time.Now()}},
	}
	s := &YouTubeSyncService{db: db}
	sources, err := s.playlistSources(context.Background(), &models.YoutubeSyncAccount{ID: "account"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0].state.SourceID != "playlist:new" {
		t.Fatal("unattempted playlists must get a turn before the feed")
	}
}
