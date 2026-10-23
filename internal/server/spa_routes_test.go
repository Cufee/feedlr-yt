package server

import (
	"io"
	"net/http"
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
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("document %s cache control = %q", path, response.Header.Get("Cache-Control"))
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
		if !strings.Contains(response.Header.Get("Cache-Control"), "no-store") {
			t.Errorf("JSON %s %s cache control = %q", item.method, item.path, response.Header.Get("Cache-Control"))
		}
	}
}

func TestSPAAssetCachePolicyAcrossDeployments(t *testing.T) {
	t.Setenv("FEEDLR_VITE_URL", "")
	t.Setenv("MAINTENANCE_MODE", "")
	for _, build := range []struct{ name, current, absent string }{
		{"previous", "123", "789"},
		{"next", "789", "123"},
	} {
		t.Run(build.name, func(t *testing.T) {
			assets := frontendFixture()
			if build.current != "123" {
				assets["assets/ui/manifest.json"].Data = []byte(strings.ReplaceAll(string(assets["assets/ui/manifest.json"].Data), "app-123", "app-"+build.current))
				for _, ext := range []string{"js", "css"} {
					assets["assets/ui/assets/app-"+build.current+"."+ext] = assets["assets/ui/assets/app-123."+ext]
					delete(assets, "assets/ui/assets/app-123."+ext)
				}
			}
			assets["assets/favicon.png"] = &fstest.MapFile{Data: []byte("icon")}
			app, closePlayback, err := newApp(nil, nil, assets, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer closePlayback()
			document, err := app.Test(httptest.NewRequest(http.MethodGet, "/app", nil))
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(document.Body)
			document.Body.Close()
			if document.StatusCode != http.StatusOK || document.Header.Get("Cache-Control") != "no-store" || !strings.Contains(string(body), "/ui-assets/assets/app-"+build.current+".js") || strings.Contains(string(body), "/ui-assets/assets/app-"+build.absent+".js") {
				t.Fatalf("document did not select its own build: status = %d, headers = %v, body = %s", document.StatusCode, document.Header, body)
			}
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				for _, item := range []struct {
					path, policy string
					status       int
				}{
					{"assets/app-" + build.current + ".js", "public, max-age=31536000, immutable", http.StatusOK},
					{"assets/app-" + build.current + ".css", "public, max-age=31536000, immutable", http.StatusOK},
					{"assets/app-" + build.absent + ".js", "no-store", http.StatusNotFound},
					{"assets/app-" + build.absent + ".css", "no-store", http.StatusNotFound},
					{".private.js", "no-store", http.StatusNotFound},
					{"manifest.json", "no-cache", http.StatusOK},
				} {
					response, err := app.Test(httptest.NewRequest(method, "/ui-assets/"+item.path, nil))
					if err != nil {
						t.Fatal(err)
					}
					response.Body.Close()
					if response.StatusCode != item.status || response.Header.Get("Location") != "" || strings.Contains(response.Header.Get("Content-Type"), "text/html") {
						t.Fatalf("%s %s: status = %d, headers = %v", method, item.path, response.StatusCode, response.Header)
					}
					assertFrontendAssetCachePolicy(t, response, item.policy)
				}
			}
			for _, method := range []string{http.MethodPost, http.MethodOptions} {
				response, err := app.Test(httptest.NewRequest(method, "/ui-assets/assets/app-"+build.current+".js", nil))
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusMethodNotAllowed || response.Header.Get("Allow") != "GET, HEAD" {
					t.Fatalf("%s asset: status = %d, headers = %v", method, response.StatusCode, response.Header)
				}
				assertFrontendAssetCachePolicy(t, response, "no-store")
			}
		})
	}
}
