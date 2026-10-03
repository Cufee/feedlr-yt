package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/server/handler"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/cufee/tpot"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

type tvRouteSessions struct {
	database.SessionsClient
}

func (tvRouteSessions) GetSession(context.Context, string) (*models.Session, error) {
	return &models.Session{ID: "session", UserID: null.StringFrom("user"), ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func newTVRouteTestApp(t *testing.T, authenticated bool) *fiber.App {
	t.Helper()
	client, err := sessions.New(tvRouteSessions{})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	if authenticated {
		session, err := client.Get(context.Background(), "session")
		if err != nil {
			t.Fatal(err)
		}
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("session", session)
			return c.Next()
		})
	}
	newContext := handler.NewBuilder(nil, client, nil, nil)
	// Exercise the production tpot/Fiber adapter, including response writing.
	toFiber := func(endpoint tpot.Servable[*handler.Context]) fiber.Handler {
		return func(c *fiber.Ctx) error {
			return adaptor.HTTPHandler(endpoint.Handler(newContext(c)))(c)
		}
	}
	app.Get("/api/tv/status", toFiber(TVPlayerStatus))
	app.Post("/api/videos/:id/tv", toFiber(SendVideoToTV))
	return app
}

func TestTVRoutes(t *testing.T) {
	previousService := logic.DefaultYouTubeTVSync
	logic.DefaultYouTubeTVSync = nil
	t.Cleanup(func() { logic.DefaultYouTubeTVSync = previousService })

	for _, tc := range []struct {
		name          string
		method        string
		path          string
		body          string
		authenticated bool
		service       *logic.YouTubeTVSyncService
		status        int
		wantError     string
		wantOffline   bool
	}{
		{name: "status requires authentication", method: http.MethodGet, path: "/api/tv/status", status: http.StatusUnauthorized},
		{name: "send requires authentication", method: http.MethodPost, path: "/api/videos/abcdefghijk/tv", body: `{"position":0}`, status: http.StatusUnauthorized},
		{name: "disabled service returns offline JSON", method: http.MethodGet, path: "/api/tv/status", authenticated: true, status: http.StatusOK, wantOffline: true},
		{name: "missing position", body: `{}`, wantError: "Invalid video position"},
		{name: "null position", body: `{"position":null}`, wantError: "Invalid video position"},
		{name: "string position", body: `{"position":"10"}`, wantError: "Invalid video position"},
		{name: "malformed JSON", body: `{"position":`, wantError: "Invalid video position"},
		{name: "negative position", body: `{"position":-1}`, service: &logic.YouTubeTVSyncService{}, wantError: "Invalid video or position"},
		{name: "position exceeds limit", body: `{"position":2147483648}`, service: &logic.YouTubeTVSyncService{}, wantError: "Invalid video or position"},
		{name: "invalid video ID", path: "/api/videos/invalid/tv", body: `{"position":0}`, service: &logic.YouTubeTVSyncService{}, wantError: "Invalid video or position"},
		{name: "offline send", method: http.MethodPost, path: "/api/videos/abcdefghijk/tv", body: `{"position":12.5}`, authenticated: true, status: http.StatusConflict, wantError: "TV is offline"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.method == "" {
				tc.method = http.MethodPost
				tc.authenticated = true
				tc.status = http.StatusBadRequest
			}
			if tc.path == "" {
				tc.path = "/api/videos/abcdefghijk/tv"
			}
			logic.DefaultYouTubeTVSync = tc.service
			app := newTVRouteTestApp(t, tc.authenticated)
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d; want %d", response.StatusCode, tc.status)
			}
			if !tc.wantOffline && tc.wantError == "" {
				return
			}
			if !strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
				t.Errorf("Content-Type = %q; want application/json", response.Header.Get("Content-Type"))
			}
			var payload struct {
				Online     *bool  `json:"online"`
				ScreenName string `json:"screenName"`
				Error      string `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response JSON: %v", err)
			}
			if tc.wantOffline {
				if payload.Online == nil || *payload.Online || payload.ScreenName != "" {
					t.Errorf("expected offline status without a screen name: %+v", payload)
				}
				if response.Header.Get("Cache-Control") != "private, no-store" {
					t.Errorf("Cache-Control = %q; want private, no-store", response.Header.Get("Cache-Control"))
				}
			}
			if payload.Error != tc.wantError {
				t.Errorf("error = %q; want %q", payload.Error, tc.wantError)
			}
		})
	}
}
