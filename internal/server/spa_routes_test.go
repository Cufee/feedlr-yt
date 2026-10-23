package server

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPADocumentNavigationAndJSONBoundary(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	t.Setenv("MAINTENANCE_MODE", "")
	assets := frontendFixture()
	assets["assets/favicon.png"] = &fstest.MapFile{Data: []byte("icon")}
	app, closePlayback, err := newApp(nil, nil, assets, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closePlayback()
	for _, path := range []string{"/app", "/app/recent?source=podcasts&page=2", "/app/watch-later?page=2", "/app/playlist/collection", "/app/settings/podcasts", "/login?return=%2Fapp", "/channel/creator", "/video/abcdefghijk"} {
		response, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || !strings.Contains(response.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), `id="root"`) || response.Header.Get("Location") != "" {
			t.Errorf("document %s: status=%d headers=%v", path, response.StatusCode, response.Header)
		}
	}
	for _, item := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/session", 200},
		{"GET", "/api/v1/feed", 401},
		{"GET", "/api/retired", 404},
		{"POST", "/api/playlists", 404},
		{"POST", "/api/videos/abcdefghijk/watch-later", 404},
		{"POST", "/login/begin", 405},
	} {
		response, err := app.Test(httptest.NewRequest(item.method, item.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != item.status || !strings.Contains(response.Header.Get("Content-Type"), "application/json") || response.Header.Get("Location") != "" || response.Header.Get("HX-Redirect") != "" {
			t.Errorf("JSON %s %s: status=%d headers=%v", item.method, item.path, response.StatusCode, response.Header)
		}
	}
}
