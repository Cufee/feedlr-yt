package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/playback"
	"github.com/gofiber/fiber/v2"
)

type fakePlayback struct {
	resolves int
	open     func(context.Context, string, string, string, string, string) (*http.Response, error)
}

func (f *fakePlayback) Resolve(context.Context, string, string) playback.Result {
	f.resolves++
	return playback.Result{Mode: "native", Reason: "ready", ManifestURL: "/api/playback/token/manifest.mpd"}
}
func (f *fakePlayback) ResolveAudio(ctx context.Context, user, video string) playback.Result {
	result := f.Resolve(ctx, user, video)
	result.AudioOnly = true
	return result
}
func (f *fakePlayback) Manifest(user, session string) ([]byte, error) {
	if user != "owner" {
		return nil, playback.ErrForbidden
	}
	return []byte("<MPD/>"), nil
}
func (f *fakePlayback) OpenMedia(ctx context.Context, user, session, resource, method, byteRange string) (*http.Response, error) {
	return f.open(ctx, user, session, resource, method, byteRange)
}

type fakePlaybackDB struct {
	progress int64
	duration int64
	kind     string
}

func (d *fakePlaybackDB) GetUserViews(context.Context, string, ...string) ([]*models.View, error) {
	return []*models.View{{VideoID: "abcdefghijk", Progress: d.progress}}, nil
}
func (d *fakePlaybackDB) GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error) {
	return &models.Video{Duration: d.duration, Type: d.kind}, nil
}

func playbackTestApp(r playbackRoutes) *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error { c.Locals("playback_user", "owner"); return c.Next() })
	app.Post("/api/videos/:id/playback", r.resolve)
	app.Get("/api/playback/:session/media/:resource", r.media)
	return app
}

func TestPlaybackResolutionUsesFreshProgressAndManualBypass(t *testing.T) {
	service := &fakePlayback{}
	db := &fakePlaybackDB{progress: 123, duration: 600, kind: "video"}
	app := playbackTestApp(playbackRoutes{service, db})
	for _, tc := range []struct {
		body     string
		progress int64
		want     string
	}{
		{`{"mode":"iframe"}`, 123, `"progress":123`},
		{`{"mode":"iframe"}`, 234, `"progress":234`},
		{`{"mode":"iframe"}`, 600, `"progress":0`},
	} {
		db.progress = tc.progress
		req := httptest.NewRequest("POST", "/api/videos/abcdefghijk/playback", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(string(body), tc.want) {
			t.Fatalf("%d %s", res.StatusCode, body)
		}
	}
	if service.resolves != 0 {
		t.Fatal("manual iframe contacted Companion")
	}
	db.kind = "live_stream"
	req := httptest.NewRequest("POST", "/api/videos/abcdefghijk/playback", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "live_or_upcoming") || service.resolves != 0 {
		t.Fatalf("live contacted Companion: %s", body)
	}
}

func TestPlaybackRoutesRequireAuthentication(t *testing.T) {
	app := fiber.New()
	registerPlaybackRoutes(app, &fakePlayback{}, &fakePlaybackDB{}, nil)
	for _, path := range []string{"/api/playback/token/manifest.mpd", "/api/playback/token/media/0"} {
		res, err := app.Test(httptest.NewRequest("GET", path, nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
}

type trackedBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *trackedBody) Close() error { b.closed.Store(true); return nil }

func TestNativeStreamPreservesRangesAndCloses(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader("abcd")}
	var requestContext context.Context
	service := &fakePlayback{open: func(ctx context.Context, user, session, resource, method, byteRange string) (*http.Response, error) {
		requestContext = ctx
		if user != "owner" || session != "token" || resource != "0" || byteRange != "bytes=4-7" {
			t.Errorf("incorrect forwarding")
		}
		return &http.Response{StatusCode: 206, ContentLength: 4, Header: http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 4-7/10"}, "Accept-Ranges": {"bytes"}}, Body: body}, nil
	}}
	app := playbackTestApp(playbackRoutes{service, nil})
	req := httptest.NewRequest("GET", "/api/playback/token/media/0", nil)
	req.Header.Set("Range", "bytes=4-7")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != 206 || string(data) != "abcd" || res.ContentLength != 4 || res.Header.Get("Content-Range") != "bytes 4-7/10" || res.Header.Get("Content-Type") != "video/mp4" {
		t.Fatalf("bad stream: %+v %s", res, data)
	}
	if !body.closed.Load() || !errors.Is(requestContext.Err(), context.Canceled) {
		t.Fatal("upstream not closed and canceled")
	}
}

// Real sockets exercise fasthttp's streaming path; app.Test buffers its result.
// The producer is effectively unbounded, so a buffering adapter would hang.
func TestNativeStreamDisconnectCancelsWithoutBuffering(t *testing.T) {
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(upstreamCanceled)
		chunk := strings.Repeat("x", 32768)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		}
	}))
	defer upstream.Close()
	service := &fakePlayback{open: func(ctx context.Context, _, _, _, _, _ string) (*http.Response, error) {
		req, _ := http.NewRequestWithContext(ctx, "GET", upstream.URL, nil)
		return http.DefaultClient.Do(req)
	}}
	app := playbackTestApp(playbackRoutes{service, nil})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(listener) }()
	defer app.Shutdown()
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://" + listener.Addr().String() + "/api/playback/token/media/0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.ReadFull(res.Body, make([]byte, 1024)); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	select {
	case <-upstreamCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream did not stop after downstream disconnect")
	}
}
