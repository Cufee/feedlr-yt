package playback

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "1234567890abcdef"

type fakeCompanion struct {
	server       *httptest.Server
	manifests    atomic.Int64
	probes       atomic.Int64
	videoMedia   atomic.Int64
	fail         atomic.Bool
	expired      atomic.Bool
	invalid      atomic.Bool
	stall        atomic.Bool
	disconnected chan struct{}
}

func newFake(t *testing.T) *fakeCompanion {
	t.Helper()
	f := &fakeCompanion{disconnected: make(chan struct{}, 16)}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testSecret {
			http.Error(w, "bad auth", 401)
			return
		}
		if strings.Contains(r.URL.Path, "/manifest/") {
			f.manifests.Add(1)
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			check, err := base64.URLEncoding.DecodeString(r.URL.Query().Get("check"))
			if err != nil || len(check)%16 != 0 || len(check) == 0 {
				http.Error(w, "bad check", 403)
				return
			}
			cipher, _ := aes.NewCipher([]byte(testSecret))
			plain := make([]byte, len(check))
			for i := 0; i < len(check); i += 16 {
				cipher.Decrypt(plain[i:i+16], check[i:i+16])
			}
			if !strings.Contains(string(plain), "|"+id) {
				http.Error(w, "bad signature", 403)
				return
			}
			if f.fail.Load() || id == "missing0000" {
				http.Error(w, "unavailable", 503)
				return
			}
			if f.invalid.Load() {
				fmt.Fprint(w, "<MPD><bad>")
				return
			}
			exp := time.Now().Add(time.Hour).Unix()
			if f.expired.Load() {
				exp = time.Now().Add(-time.Minute).Unix()
			}
			w.Header().Set("Content-Type", "application/dash+xml")
			fmt.Fprint(w, fixtureManifest(exp))
			return
		}
		if r.URL.Path != "/companion/videoplayback" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("mime") == "video/mp4" {
			f.videoMedia.Add(1)
		}
		if f.fail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		var first, last int64
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &first, &last); err != nil {
			first = 0
			last = 65535
		}
		if last < first {
			w.WriteHeader(416)
			return
		}
		if last-first == 65535 {
			f.probes.Add(1)
		}
		w.Header().Set("Content-Type", r.URL.Query().Get("mime"))
		w.Header().Set("Content-Length", strconv.FormatInt(last-first+1, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/1048576", first, last))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(206)
		if f.stall.Load() {
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			select {
			case f.disconnected <- struct{}{}:
			default:
			}
			return
		}
		data := make([]byte, last-first+1)
		copy(data[4:], "ftyp")
		_, _ = w.Write(data)
	}))
	t.Cleanup(f.server.Close)
	return f
}
func fixtureManifest(exp int64) string {
	return fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT60S"><Period><AdaptationSet mimeType="audio/mp4"><Representation id="140" bandwidth="128000" codecs="mp4a.40.2"><BaseURL>/companion/videoplayback?expire=%d&amp;host=rr1---sn-test.googlevideo.com&amp;mime=audio/mp4&amp;secret=hidden</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation></AdaptationSet><AdaptationSet mimeType="video/mp4"><Representation id="137" bandwidth="1000000" codecs="avc1.640028" width="1920" height="1080"><BaseURL>/companion/videoplayback?expire=%d&amp;host=rr1---sn-test.googlevideo.com&amp;mime=video/mp4</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation></AdaptationSet></Period></MPD>`, exp, exp)
}
func newService(t *testing.T, f *fakeCompanion) *Service {
	t.Helper()
	s, err := New(Config{CompanionURL: f.server.URL, CompanionKey: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func resolvedSession(t *testing.T, s *Service) string {
	t.Helper()
	r := s.Resolve(context.Background(), "user", "dQw4w9WgXcQ")
	if r.Mode != "native" {
		t.Fatalf("resolve: %+v", r)
	}
	return strings.Split(r.ManifestURL, "/")[3]
}

func TestManifestRewriteAuthorizationAndRanges(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	id := resolvedSession(t, s)
	data, err := s.Manifest("user", id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "googlevideo") || strings.Contains(string(data), "secret") || strings.Contains(string(data), "expire=") {
		t.Fatal("upstream details leaked")
	}
	if strings.Count(string(data), `xmlns=`) != 1 {
		t.Fatal("invalid duplicate XML namespace")
	}
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Periods[0].Sets) != 2 {
		t.Fatal("lost streams")
	}
	if _, err := s.Manifest("other", id); !errors.Is(err, ErrForbidden) {
		t.Fatalf("authorization: %v", err)
	}
	if _, err := s.Manifest("user", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := s.OpenMedia(context.Background(), "other", id, "0", "GET", ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("media authorization: %v", err)
	}
	if _, err := s.OpenMedia(context.Background(), "user", id, "0", "GET", "bytes=0-1,4-5"); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("multi-range: %v", err)
	}
	r, err := s.OpenMedia(context.Background(), "user", id, "1", "GET", "bytes=100000-100127")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 206 || r.ContentLength != 128 || len(body) != 128 || r.Header.Get("Content-Range") != "bytes 100000-100127/1048576" {
		t.Fatal("range semantics changed")
	}
	f.fail.Store(true)
	r, err = s.OpenMedia(context.Background(), "user", id, "1", "GET", "bytes=0-100")
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil || r.StatusCode != 503 || string(body) != "Media unavailable\n" || r.ContentLength != int64(len(body)) {
		t.Fatal("unsafe upstream error response")
	}
	s.mu.Lock()
	s.sessions[id].ExpiresAt = time.Now().Add(-time.Second)
	s.mu.Unlock()
	if _, err := s.Manifest("user", id); !errors.Is(err, ErrExpired) {
		t.Fatalf("expiry: %v", err)
	}
}

func TestManifestRejectsUnsafeOrUnusableURLs(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	valid := fixtureManifest(time.Now().Add(time.Hour).Unix())
	for name, bad := range map[string]string{
		"foreign origin":          strings.ReplaceAll(valid, "/companion/videoplayback", "http://169.254.169.254/companion/videoplayback"),
		"proxy target":            strings.ReplaceAll(valid, "rr1---sn-test.googlevideo.com", "127.0.0.1"),
		"suffix attack":           strings.ReplaceAll(valid, "rr1---sn-test.googlevideo.com", "googlevideo.com.attacker.test"),
		"external initialization": strings.ReplaceAll(valid, `range="0-99"`, `range="0-99" sourceURL="https://attacker.test"`),
		"template":                strings.ReplaceAll(valid, "<SegmentBase", `<SegmentTemplate media="https://attacker.test"/><SegmentBase`),
		"dynamic":                 strings.ReplaceAll(valid, `type="static"`, `type="dynamic"`),
		"expired":                 fixtureManifest(time.Now().Add(-time.Minute).Unix()),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.parseManifest([]byte(bad)); err == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestHealthCoalescingCircuitRecoveryAndIndividualFailures(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !s.healthy(context.Background()) {
				t.Error("health failed")
			}
		}()
	}
	wg.Wait()
	if f.manifests.Load() != 1 || f.probes.Load() != 2 {
		t.Fatalf("check was not shared: manifests=%d probes=%d", f.manifests.Load(), f.probes.Load())
	}
	r := s.Resolve(context.Background(), "user", "missing0000")
	if r.Mode != "iframe" {
		t.Fatal("missing video accepted")
	}
	if !s.healthy(context.Background()) {
		t.Fatal("individual failure opened circuit")
	}
	f.fail.Store(true)
	s.mu.Lock()
	s.health.validUntil = time.Time{}
	s.mu.Unlock()
	if s.healthy(context.Background()) {
		t.Fatal("outage healthy")
	}
	n := f.manifests.Load()
	if s.healthy(context.Background()) || f.manifests.Load() != n {
		t.Fatal("open circuit retried immediately")
	}
	f.fail.Store(false)
	s.mu.Lock()
	s.health.nextRetry = time.Time{}
	s.mu.Unlock()
	if !s.healthy(context.Background()) {
		t.Fatal("failed recovery")
	}
	if f.manifests.Load() != n+2 {
		t.Fatal("recovery did not verify both videos")
	}
}

func TestProbeRejectsFakeOrTruncatedMedia(t *testing.T) {
	for _, tc := range []struct {
		name, kind, cr, body string
		status               int
		length               int64
	}{
		{"html", "text/html", "bytes 0-65535/99999", strings.Repeat("x", 65536), 206, 65536},
		{"fake media", "video/mp4", "bytes 0-65535/99999", strings.Repeat("x", 65536), 206, 65536},
		{"ignored range", "video/mp4", "bytes 0-65535/99999", strings.Repeat("x", 65536), 200, 65536},
		{"wrong range", "video/mp4", "bytes 1-65536/99999", strings.Repeat("x", 65536), 206, 65536},
		{"truncated", "video/mp4", "bytes 0-65535/99999", "ftyp", 206, 65536},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Response{StatusCode: tc.status, ContentLength: tc.length, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.body))}
			r.Header.Set("Content-Type", tc.kind)
			r.Header.Set("Content-Range", tc.cr)
			if validateProbe(r, 0) == nil {
				t.Fatal("invalid probe accepted")
			}
		})
	}
}

func TestStreamingCancellationAndConcurrency(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	id := resolvedSession(t, s)
	f.stall.Store(true)
	var bodies []io.ReadCloser
	for i := 0; i < 8; i++ {
		r, err := s.OpenMedia(context.Background(), "user", id, "0", "GET", "bytes=0-65535")
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, r.Body)
	}
	if _, err := s.OpenMedia(context.Background(), "user", id, "0", "GET", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected concurrency bound: %v", err)
	}
	for _, b := range bodies {
		b.Close()
		b.Close()
	}
	select {
	case <-f.disconnected:
	case <-time.After(time.Second):
		t.Fatal("body close did not cancel upstream")
	}
	s.mu.Lock()
	count := s.mediaCount
	s.mu.Unlock()
	if count != 0 {
		t.Fatalf("leaked media slots: %d", count)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r, err := s.OpenMedia(ctx, "user", id, "0", "GET", "bytes=0-65535")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_, err = r.Body.Read(make([]byte, 1))
	r.Body.Close()
	if err == nil {
		t.Fatal("canceled context still reads")
	}
}

func TestExpiredExtractionAndBypass(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	if r := s.Resolve(context.Background(), "", "dQw4w9WgXcQ"); r.Reason != "guest" {
		t.Fatal(r)
	}
	s.config.ForceIframe = true
	if r := s.Resolve(context.Background(), "user", "dQw4w9WgXcQ"); r.Reason != "disabled" {
		t.Fatal(r)
	}
	s.config.ForceIframe = false
	if f.manifests.Load() != 0 {
		t.Fatal("bypass contacted Companion")
	}
	resolvedSession(t, s)
	f.expired.Store(true)
	if r := s.Resolve(context.Background(), "user", "dQw4w9WgXcQ"); r.Reason != "expired_urls" {
		t.Fatalf("unchanged expired URLs accepted: %+v", r)
	}
	f.expired.Store(false)
	f.invalid.Store(true)
	if r := s.Resolve(context.Background(), "user", "dQw4w9WgXcQ"); r.Reason != "invalid_manifest" {
		t.Fatalf("invalid manifest accepted: %+v", r)
	}
}

func TestResolverHonorsCallerDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	s, err := New(Config{CompanionURL: server.URL, CompanionKey: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	if r := s.Resolve(ctx, "user", "dQw4w9WgXcQ"); r.Reason != "resolution_timeout" {
		t.Fatal(r)
	}
	if time.Since(start) > time.Second {
		t.Fatal("caller deadline ignored")
	}
}

func TestAudioOnlySessionNeverExposesVideoMedia(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	// Warm the shared health check: its bounded audio/video probes are separate
	// from the user's audio session and run regardless of playback mode.
	videoID := resolvedSession(t, s)
	videoRequests := f.videoMedia.Load()
	r := s.ResolveAudio(context.Background(), "user", "dQw4w9WgXcQ")
	if r.Mode != "native" || !r.AudioOnly {
		t.Fatalf("audio resolution: %+v", r)
	}
	if len(r.Qualities) != 1 || r.Qualities[0] != (Quality{Width: 1920, Height: 1080}) {
		t.Fatalf("missing video choices: %+v", r.Qualities)
	}
	id := strings.Split(r.ManifestURL, "/")[3]
	data, err := s.Manifest("user", id)
	if err != nil {
		t.Fatal(err)
	}
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Periods) != 1 || len(m.Periods[0].Sets) != 1 || m.Periods[0].Sets[0].MIME != "audio/mp4" {
		t.Fatalf("audio manifest contains video: %s", data)
	}
	if got := m.Periods[0].Sets[0].Representations[0].BaseURL; got != "/api/playback/"+id+"/media/0" {
		t.Fatalf("incorrect audio resource mapping: %s", got)
	}
	response, err := s.OpenMedia(context.Background(), "user", id, "0", "GET", "bytes=0-65535")
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if err != nil || response.Header.Get("Content-Type") != "audio/mp4" {
		t.Fatalf("audio resource mismatch: %v", err)
	}
	if _, err := s.OpenMedia(context.Background(), "user", id, "1", "GET", "bytes=0-65535"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("audio session exposed video resource: %v", err)
	}
	if f.videoMedia.Load() != videoRequests {
		t.Fatal("audio playback transferred video bytes")
	}
	// Filtering one session must not remove video from existing sessions.
	videoData, err := s.Manifest("user", videoID)
	if err != nil || !strings.Contains(string(videoData), "video/mp4") {
		t.Fatal("audio filtering changed a video session")
	}
}

func TestAudioFilteringRemapsInterleavedResourcesWithoutMutatingExtraction(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	r, err := s.parseManifest([]byte(fixtureManifest(time.Now().Add(time.Hour).Unix())))
	if err != nil {
		t.Fatal(err)
	}
	// Exercise video-first input and distinct codecs sharing one resolution.
	audio, video := r.MPD.Periods[0].Sets[0], r.MPD.Periods[0].Sets[1]
	audioResource, videoResource := r.Resources[0], r.Resources[1]
	low := video
	low.Representations = append([]representation(nil), video.Representations...)
	low.Representations[0].Width, low.Representations[0].Height = 640, 360
	r.MPD.Periods[0].Sets = []adaptation{low, video, audio, video, audio}
	r.Resources = []resource{videoResource, videoResource, audioResource, videoResource, audioResource}
	qualities := r.qualities()
	if len(qualities) != 2 || qualities[0].Height != 1080 || qualities[1].Height != 360 {
		t.Fatalf("incorrect distinct ordered qualities: %+v", qualities)
	}
	only := r.audioOnly()
	if len(only.Resources) != 2 || len(only.MPD.Periods[0].Sets) != 2 {
		t.Fatal("audio tracks lost")
	}
	data, err := only.manifest("test-session")
	if err != nil {
		t.Fatal(err)
	}
	for i, res := range only.Resources {
		if res.Kind != "audio" || res.URL != audioResource.URL || !strings.Contains(string(data), fmt.Sprintf("/api/playback/test-session/media/%d", i)) {
			t.Fatal("audio resource incorrectly remapped")
		}
	}
	if len(r.Resources) != 5 || len(r.MPD.Periods[0].Sets) != 5 || r.Resources[0].Kind != "video" {
		t.Fatal("shared extraction mutated")
	}
}
