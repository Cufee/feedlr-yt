package webapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/null/v9"
	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

type fixture struct {
	app          *fiber.App
	db           database.Client
	session      string
	otherSession string
	schemas      map[string]any
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	previousSponsor, previousYouTube := sponsorblock.C, youtube.DefaultClient
	sponsorblock.C = nil
	youtube.DefaultClient = nil
	t.Cleanup(func() { sponsorblock.C = previousSponsor; youtube.DefaultClient = previousYouTube })
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "api.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob("../../database/migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("migrations: %v", err)
	}
	for _, p := range migrations {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = raw.Exec(string(b)); err != nil {
			t.Fatalf("migration %s: %v", p, err)
		}
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := database.NewSQLiteClient(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, id := range []string{"viewer", "other"} {
		if _, err = db.CreateUser(ctx, id, id); err != nil {
			t.Fatal(err)
		}
	}
	channel := &models.Channel{ID: "show", Title: "A podcast", Description: "A show", Thumbnail: "https://images.example.test/art.jpg", FeedUpdatedAt: time.Now()}
	if err = db.UpsertChannel(ctx, channel); err != nil {
		t.Fatal(err)
	}
	if err = db.UpsertPodcastShow(ctx, &models.PodcastShow{ChannelID: "show", FeedURL: "https://example.test/feed.xml"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pe_first", "pe_second"} {
		if err = db.UpsertVideos(ctx, &models.Video{ID: id, ChannelID: "show", Title: "An episode", Description: `<p>Episode <strong>notes</strong></p><script>alert(1)</script>`, Type: "podcast_episode", Duration: 3600, MediaURL: null.StringFrom("https://example.test/audio.mp3"), PublishedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.NewSubscription(ctx, "viewer", "show"); err != nil {
		t.Fatal(err)
	}
	sc, err := sessions.New(db)
	if err != nil {
		t.Fatal(err)
	}
	makeSession := func(id string) string {
		s, err := sc.New(ctx)
		if err != nil {
			t.Fatal(err)
		}
		s, err = s.UpdateUser(ctx, null.StringFrom(id), null.String{})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID()
	}
	f := &fixture{app: fiber.New(), db: db, session: makeSession("viewer"), otherSession: makeSession("other")}
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "Feedlr", RPID: "example.com", RPOrigins: []string{"http://example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	Register(f.app, db, sc, wa)
	doc, err := os.ReadFile("../../../api/frontend.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err = json.Unmarshal(doc, &spec); err != nil {
		t.Fatal(err)
	}
	f.schemas = spec.Components.Schemas
	return f
}

func (f *fixture) request(t *testing.T, method, path, body, session, origin string) (*http.Response, any) {
	t.Helper()
	req := httptest.NewRequest(method, "http://example.com/api/v1"+path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if session != "" {
		req.AddCookie(&http.Cookie{Name: "session_id", Value: session})
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, err := f.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if len(b) > 0 {
		if err = json.Unmarshal(b, &value); err != nil {
			t.Fatalf("%s %s returned non-JSON %d: %s", method, path, res.StatusCode, b)
		}
	}
	if res.StatusCode != 204 && !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("non-JSON content type: %v", res.Header)
	}
	if res.Header.Get("Location") != "" || res.Header.Get("HX-Redirect") != "" {
		t.Fatalf("JSON request redirected: %v", res.Header)
	}
	return res, value
}

func (f *fixture) check(t *testing.T, value any, schema string) {
	t.Helper()
	if err := validate(value, f.schemas[schema], f.schemas); err != nil {
		t.Fatalf("response violates %s: %v\n%#v", schema, err, value)
	}
}

// Validate the schema constructs used in this contract against actual HTTP
// output, including discriminated variants and accidental null arrays.
func validate(v, definition any, schemas map[string]any) error {
	s := definition.(map[string]any)
	if name, ok := s["$ref"].(string); ok {
		return validate(v, schemas[strings.TrimPrefix(name, "#/components/schemas/")], schemas)
	}
	if alternatives, ok := s["oneOf"].([]any); ok {
		matches := 0
		for _, a := range alternatives {
			if validate(v, a, schemas) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("oneOf matched %d variants", matches)
		}
		return nil
	}
	if values, ok := s["enum"].([]any); ok {
		found := false
		for _, e := range values {
			if e == v {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%v is outside enum", v)
		}
	}
	if alternatives, ok := s["type"].([]any); ok {
		for _, typ := range alternatives {
			clone := map[string]any{}
			for k, x := range s {
				clone[k] = x
			}
			clone["type"] = typ
			if validate(v, clone, schemas) == nil {
				return nil
			}
		}
		return fmt.Errorf("invalid nullable value %v", v)
	}
	switch s["type"] {
	case "null":
		if v != nil {
			return fmt.Errorf("expected null")
		}
	case "string":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", v)
		}
	case "integer", "number":
		n, ok := v.(float64)
		if !ok {
			return fmt.Errorf("expected number, got %T", v)
		}
		if s["type"] == "integer" && n != math.Trunc(n) {
			return fmt.Errorf("expected integer")
		}
	case "array":
		list, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %T", v)
		}
		for _, item := range list {
			if err := validate(item, s["items"], schemas); err != nil {
				return err
			}
		}
	case "object":
		o, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object, got %T", v)
		}
		properties, _ := s["properties"].(map[string]any)
		if required, ok := s["required"].([]any); ok {
			for _, key := range required {
				if _, exists := o[key.(string)]; !exists {
					return fmt.Errorf("missing %s", key)
				}
			}
		}
		for key, x := range o {
			if p, exists := properties[key]; exists {
				if err := validate(x, p, schemas); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			} else if s["additionalProperties"] == false {
				return fmt.Errorf("unexpected %s", key)
			}
		}
	}
	return nil
}

func TestAPIReadsUseThePublishedWireContract(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/feed", "/history", "/watch-later", "/subscriptions", "/playlists", "/settings"} {
		res, v := f.request(t, "GET", path, "", f.session, "")
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d %#v", path, res.StatusCode, v)
		}
		schema := map[string]string{"/feed": "Feed", "/history": "MediaList", "/watch-later": "MediaList", "/subscriptions": "ChannelList", "/playlists": "PlaylistList", "/settings": "Settings"}[path]
		f.check(t, v, schema)
	}
	for _, path := range []string{"/media/pe_first", "/channels/show"} {
		res, v := f.request(t, "GET", path, "", "", "")
		if res.StatusCode != 200 {
			t.Fatalf("guest %s: %d %#v", path, res.StatusCode, v)
		}
		schema := "MediaDetail"
		if strings.HasPrefix(path, "/channels") {
			schema = "ChannelDetail"
		}
		f.check(t, v, schema)
	}
	res, v := f.request(t, "GET", "/media/pe_first", "", "", "")
	detail := v.(map[string]any)
	item := detail["item"].(map[string]any)
	if res.StatusCode != 200 || detail["authenticated"] != false || detail["reportProgress"] != false || item["kind"] != "podcast" || item["streamUrl"] != "/media/stream/pe_first" || strings.Contains(item["descriptionHtml"].(string), "<script") {
		t.Fatalf("guest media/notes contract: %#v", v)
	}
	res, v = f.request(t, "GET", "/session", "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	f.check(t, v, "Session")
}

func TestAPIExplicitIntentVisibilityCompletionAndSettings(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"pe_first", "pe_second", "pe_second"} {
		res, v := f.request(t, "PUT", "/watch-later/"+id, `{"present":true}`, f.session, "http://example.com")
		if res.StatusCode != 200 {
			t.Fatalf("pin: %d %#v", res.StatusCode, v)
		}
		f.check(t, v, "WatchLaterMembership")
	}
	res, v := f.request(t, "GET", "/watch-later?limit=1&offset=0", "", f.session, "")
	f.check(t, v, "MediaList")
	if res.StatusCode != 200 || v.(map[string]any)["page"].(map[string]any)["hasMore"] != true {
		t.Fatalf("pagination: %#v", v)
	}
	for _, body := range []string{`{"hidden":true}`, `{"progressSeconds":3300}`} {
		res, v = f.request(t, "PATCH", "/media/pe_first/state", body, f.session, "http://example.com")
		if res.StatusCode != 204 {
			t.Fatalf("state: %d %#v", res.StatusCode, v)
		}
	}
	res, v = f.request(t, "GET", "/media/pe_first", "", f.session, "")
	f.check(t, v, "MediaDetail")
	item := v.(map[string]any)["item"].(map[string]any)
	if res.StatusCode != 200 || item["hidden"] != true || item["completed"] != true || item["inWatchLater"] != false {
		t.Fatalf("progress/visibility/completion: %#v", v)
	}
	for range 2 {
		res, v = f.request(t, "PATCH", "/settings", `{"sponsorBlock":{"enabled":false,"categories":[]}}`, f.session, "")
		if res.StatusCode != 204 {
			t.Fatalf("preferences: %d %#v", res.StatusCode, v)
		}
	}
	res, v = f.request(t, "PATCH", "/media/pe_first/state", `{"volume":0}`, f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("muted volume: %#v", v)
	}
	_, v = f.request(t, "GET", "/settings", "", f.session, "")
	f.check(t, v, "Settings")
	settings := v.(map[string]any)
	if settings["volume"] != float64(0) || settings["sponsorBlock"].(map[string]any)["enabled"] != false || len(settings["sponsorBlock"].(map[string]any)["categories"].([]any)) != 0 {
		t.Fatalf("partial preferences overwritten: %#v", v)
	}
}

func TestAPIPlaylistOwnershipAndRealMutations(t *testing.T) {
	f := newFixture(t)
	res, v := f.request(t, "POST", "/playlists", `{"name":"My queue","description":"A collection"}`, f.session, "")
	if res.StatusCode != 201 {
		t.Fatalf("create: %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "Playlist")
	id := v.(map[string]any)["id"].(string)
	res, v = f.request(t, "PATCH", "/playlists/"+id, `{"name":"Stolen"}`, f.otherSession, "")
	if res.StatusCode != 404 {
		t.Fatalf("cross-user write: %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "Error")
	for range 2 {
		res, v = f.request(t, "PUT", "/playlists/"+id+"/items/pe_first", `{"present":true}`, f.session, "")
		if res.StatusCode != 204 {
			t.Fatalf("membership: %d %#v", res.StatusCode, v)
		}
	}
	_, v = f.request(t, "GET", "/playlists/"+id, "", f.session, "")
	f.check(t, v, "PlaylistDetail")
	if len(v.(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("duplicate insertion: %#v", v)
	}
	res, v = f.request(t, "PATCH", "/playlists/"+id, `{"description":"Changed"}`, f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("edit: %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "Playlist")
	if v.(map[string]any)["name"] != "My queue" {
		t.Fatalf("omitted name replaced: %#v", v)
	}
	res, v = f.request(t, "DELETE", "/playlists/"+id, "", f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("delete: %d %#v", res.StatusCode, v)
	}
	res, v = f.request(t, "GET", "/playlists/"+id, "", f.session, "")
	if res.StatusCode != 404 {
		t.Fatalf("deleted playlist still readable: %d %#v", res.StatusCode, v)
	}
}

func TestAPIErrorsNeverRedirectAndLogoutRevokesSession(t *testing.T) {
	f := newFixture(t)
	for _, test := range []struct {
		method, path, body, session, origin string
		status                              int
	}{
		{"GET", "/watch-later", "", "", "", 401},
		{"PUT", "/watch-later/pe_first", `{"present":true}`, f.session, "https://evil.example", 403},
		{"PUT", "/watch-later/pe_first", `{`, f.session, "", 400},
		{"PUT", "/watch-later/pe_first", `{"present":true} {}`, f.session, "", 400},
		{"PUT", "/watch-later/pe_first", `{}`, f.session, "", 422},
		{"GET", "/watch-later?offset=-1", "", f.session, "", 422},
		{"GET", "/history?source=invalid", "", f.session, "", 422},
		{"PATCH", "/settings", `{"sponsorBlock":{"categories":["unknown"]}}`, f.session, "", 422},
		{"PATCH", "/media/pe_first/state", `{"progressSeconds":1,"completed":true}`, f.session, "", 422},
		{"GET", "/does-not-exist", "", f.session, "", 404},
		{"DELETE", "/passkeys/foreign", "", f.session, "", 404},
	} {
		res, v := f.request(t, test.method, test.path, test.body, test.session, test.origin)
		if res.StatusCode != test.status {
			t.Fatalf("%s %s: %d want %d (%#v)", test.method, test.path, res.StatusCode, test.status, v)
		}
		f.check(t, v, "Error")
	}
	res, v := f.request(t, "POST", "/logout", "", f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("logout: %d %#v", res.StatusCode, v)
	}
	res, v = f.request(t, "GET", "/watch-later", "", f.session, "")
	if res.StatusCode != 401 {
		t.Fatalf("revoked session accepted: %d %#v", res.StatusCode, v)
	}
}
