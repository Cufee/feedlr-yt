package server

import (
	"context"
	"database/sql"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
)

type metadataFixtureDB struct{ database.Client }

func (metadataFixtureDB) GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error) {
	return &models.Video{ID: "abcdefghijk", ChannelID: "creator", Title: "A title &amp; a story", Description: "<p>A clear &amp; useful description</p><script>alert(1)</script>", Duration: 125}, nil
}
func (metadataFixtureDB) GetChannel(context.Context, string, ...database.ChannelQuery) (*models.Channel, error) {
	return &models.Channel{ID: "creator", Title: "A creator", Description: "A channel description"}, nil
}

func TestFrontendPublicMetadataBeforeJavaScript(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	u, err := newFrontendUI(frontendFixture(), metadataFixtureDB{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/video/:id", u.page)
	app.Get("/channel/:id", u.page)
	for _, item := range []struct {
		path     string
		required []string
	}{
		{"/video/abcdefghijk?return=%2Fapp", []string{`<title data-feedlr-head>A title &amp; a story · Feedlr</title>`, `name="author" content="A creator"`, `property="video:duration" content="125"`, `property="og:video:duration" content="125"`, `itemprop="duration" content="PT125S"`, `content="A clear &amp; useful description"`, `rel="canonical" href="http://example.com/video/abcdefghijk"`, `/thumb/video/abcdefghijk/maxresdefault`}},
		{"/channel/creator", []string{`<title data-feedlr-head>A creator · Feedlr</title>`, `content="A channel description"`, `/thumb/channel/creator`}},
	} {
		response, err := app.Test(httptest.NewRequest("GET", "http://example.com"+item.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		for _, required := range item.required {
			if !strings.Contains(string(data), required) {
				t.Errorf("%s missing %s", item.path, required)
			}
		}
		if strings.Contains(string(data), "alert(1)") {
			t.Errorf("%s contains unsafe description", item.path)
		}
	}
}

func TestFrontendMetadataLookupIsBounded(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	u, err := newFrontendUI(frontendFixture(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	occupyMetadataWorkers(t, u)
	if _, ok := u.publicMetadata(context.Background(), "video", "abcdefghijk"); ok {
		t.Fatal("saturated metadata worker pool should use the default metadata")
	}
}

type metadataOutageDB struct{ metadataFixtureDB }

func (db metadataOutageDB) GetVideoByID(ctx context.Context, id string, queries ...database.VideoQuery) (*models.Video, error) {
	if id != "abcdefghijk" {
		return nil, sql.ErrNoRows
	}
	return db.metadataFixtureDB.GetVideoByID(ctx, id, queries...)
}

func occupyMetadataWorkers(t *testing.T, u *frontendUI) {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	started := make(chan struct{})
	u.metadata = func(ctx context.Context, _, _ string) (frontendMetadata, error) {
		started <- struct{}{}
		<-release
		return frontendMetadata{}, ctx.Err()
	}
	for range cap(u.metadataSlots) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			u.publicMetadata(ctx, "video", "uncached")
		}()
		<-started
		cancel()
		<-done
	}
}

func TestFrontendCachedMetadataSurvivesProviderSaturation(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	u, err := newFrontendUI(frontendFixture(), metadataOutageDB{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	occupyMetadataWorkers(t, u)
	app := fiber.New()
	app.Get("/video/:id", u.page)
	app.Get("/channel/:id", u.page)
	for _, item := range []struct{ path, title, artwork string }{
		{"/video/abcdefghijk", "A title &amp; a story", "/thumb/video/abcdefghijk/maxresdefault"},
		{"/channel/creator", "A creator", "/thumb/channel/creator"},
	} {
		response, err := app.Test(httptest.NewRequest("GET", "http://example.com"+item.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !strings.Contains(string(data), `<title data-feedlr-head>`+item.title+` · Feedlr</title>`) || !strings.Contains(string(data), `content="http://example.com`+item.artwork+`"`) {
			t.Errorf("cached metadata unavailable for %s while provider workers are occupied", item.path)
		}
		if strings.Contains(string(data), "alert(1)") {
			t.Errorf("cached metadata for %s contains unsafe description", item.path)
		}
	}
}

func frontendFixture() fstest.MapFS {
	return fstest.MapFS{
		"assets/ui/manifest.json":         {Data: []byte(`{"index.html":{"file":"assets/app-123.js","imports":["_vendor.js"],"css":["assets/app-123.css"]},"_vendor.js":{"file":"assets/vendor-456.js","css":["assets/vendor-456.css"]}}`)},
		"assets/ui/assets/app-123.js":     {Data: []byte(`import "./vendor-456.js";`)},
		"assets/ui/assets/vendor-456.js":  {Data: []byte(`export const version=1;`)},
		"assets/ui/assets/app-123.css":    {Data: []byte(`body{color:white}`)},
		"assets/ui/assets/vendor-456.css": {Data: []byte(`:root{color-scheme:dark}`)},
	}
}

func TestFrontendAssetsPreserveVitePathsAndHeaders(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	u, err := newFrontendUI(frontendFixture(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/ui-assets/*", u.asset)
	app.Head("/ui-assets/*", u.asset)
	for _, name := range []string{"app-123.js", "vendor-456.js", "app-123.css"} {
		response, err := app.Test(httptest.NewRequest("GET", "/ui-assets/assets/"+name, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || len(body) == 0 || response.Header.Get("Location") != "" {
			t.Fatalf("asset %s: status=%d location=%q body=%q", name, response.StatusCode, response.Header.Get("Location"), body)
		}
		if response.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Fatalf("asset %s missing immutable cache header", name)
		}
	}
	response, err := app.Test(httptest.NewRequest("HEAD", "/ui-assets/assets/app-123.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "javascript") || response.ContentLength != int64(len(`import "./vendor-456.js";`)) {
		t.Fatalf("HEAD headers: status=%d type=%q length=%d", response.StatusCode, response.Header.Get("Content-Type"), response.ContentLength)
	}
	missing, err := app.Test(httptest.NewRequest("GET", "/ui-assets/assets/missing.js", nil))
	if err != nil {
		t.Fatal(err)
	}
	missing.Body.Close()
	if missing.StatusCode != 404 {
		t.Fatalf("missing asset status=%d", missing.StatusCode)
	}
}

func TestFrontendManifestAndDocument(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	fixture := frontendFixture()
	u, err := newFrontendUI(fixture, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/", u.page)
	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	body := string(data)
	for _, required := range []string{`id="root"`, `/ui-assets/assets/app-123.js`, `/ui-assets/assets/vendor-456.js`, `/ui-assets/assets/vendor-456.css`, `rel="manifest"`, `property="og:title"`} {
		if !strings.Contains(body, required) {
			t.Errorf("document missing %s", required)
		}
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("personalized document must not be cached")
	}
	delete(fixture, "assets/ui/assets/vendor-456.js")
	if _, err := newFrontendUI(fixture, nil, nil); err == nil {
		t.Fatal("missing imported chunk should reject incomplete build")
	}
}

func TestFrontendMetadataEscapesUntrustedText(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return renderFrontend(c, frontendDocument{Title: `A </title><script>alert(1)</script> title`, Description: `" onload="alert(2)`, Entry: "/ui-assets/assets/app.js"})
	})
	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if strings.Contains(string(data), `<script>alert(1)`) || strings.Contains(string(data), `content="" onload=`) || !strings.Contains(string(data), `&lt;/title&gt;`) {
		t.Fatalf("unescaped metadata: %s", data)
	}
}

func TestFrontendDevelopmentAndMaintenance(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "http://127.0.0.1:5173")
	u, err := newFrontendUI(fstest.MapFS{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(u.maintenance)
	app.Get("/", u.page)
	response, err := app.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	for _, required := range []string{`http://127.0.0.1:5173/@vite/client`, `http://127.0.0.1:5173/src/main.tsx`, `__vite_plugin_react_preamble_installed__`} {
		if !strings.Contains(string(data), required) {
			t.Errorf("development document missing %s", required)
		}
	}
	t.Setenv("MAINTENANCE_MODE", "true")
	response, err = app.Test(httptest.NewRequest("GET", "/api/v1/feed", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 503 || !strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("maintenance API response: status=%d type=%q", response.StatusCode, response.Header.Get("Content-Type"))
	}
}
