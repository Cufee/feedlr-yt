// Package playback resolves private Companion manifests into user-bound,
// short-lived same-origin sessions. It never exposes extraction URLs to clients.
package playback

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	ErrNotFound        = errors.New("playback session not found")
	ErrForbidden       = errors.New("playback session forbidden")
	ErrExpired         = errors.New("playback session expired")
	ErrBusy            = errors.New("playback concurrency limit")
	ErrInvalidRange    = errors.New("invalid byte range")
	ErrInvalidManifest = errors.New("invalid playback manifest")
	videoIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	byteSpan           = regexp.MustCompile(`^[0-9]+-[0-9]+$`)
	rangePattern       = regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)
	googleVideoHost    = regexp.MustCompile(`^[a-zA-Z0-9-]+\.googlevideo\.com$`)
)

type Config struct {
	// CompanionURL is its private origin, optionally ending in /companion.
	CompanionURL string
	CompanionKey string
	ForceIframe  bool
	HTTPClient   *http.Client
	// Observe must be safe for concurrent calls; events and reasons are bounded.
	Observe func(event, reason string)
}
type Result struct {
	Mode        string    `json:"mode"`
	Reason      string    `json:"reason"`
	ManifestURL string    `json:"manifestUrl,omitempty"`
	ExpiresAt   time.Time `json:"expiresAt,omitzero"`
	Qualities   []Quality `json:"qualities,omitempty"`
	AudioOnly   bool      `json:"audioOnly,omitempty"`
}
type Quality struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}
type session struct {
	UserID    string
	Manifest  []byte
	Resources []resource
	ExpiresAt time.Time
	Created   time.Time
}
type Service struct {
	config       Config
	base         *url.URL
	client       *http.Client
	ctx          context.Context
	cancel       context.CancelFunc
	now          func() time.Time
	mu           sync.Mutex
	sessions     map[string]*session
	resolving    map[string]int
	media        map[string]int
	resolveCount int
	mediaCount   int
	flight       singleflight.Group
	extractions  chan struct{}
	health       healthState
}

func New(config Config) (*Service, error) {
	s := &Service{config: config, now: time.Now, sessions: map[string]*session{}, resolving: map[string]int{}, media: map[string]int{}, extractions: make(chan struct{}, 8)}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	if config.CompanionURL != "" {
		base, err := url.Parse(strings.TrimRight(config.CompanionURL, "/"))
		if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/companion") {
			s.cancel()
			return nil, errors.New("invalid private Companion URL")
		}
		if base.Path == "" {
			base.Path = "/companion"
		}
		s.base = base
	}
	if config.HTTPClient != nil {
		c := *config.HTTPClient
		s.client = &c
	} else {
		s.client = &http.Client{Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, MaxIdleConns: 100, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, ResponseHeaderTimeout: 8 * time.Second, TLSHandshakeTimeout: 5 * time.Second}}
	}
	// A redirect must never leak Companion credentials or turn it into an SSRF.
	s.client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	go s.healthLoop()
	return s, nil
}
func (s *Service) Close() { s.cancel(); s.client.CloseIdleConnections() }
func (s *Service) observe(event, reason string) {
	if s.config.Observe != nil {
		s.config.Observe(event, reason)
	}
}
func (s *Service) fallback(reason string) Result {
	s.observe("resolution", reason)
	return Result{Mode: "iframe", Reason: reason}
}

func (s *Service) Resolve(ctx context.Context, userID, videoID string) Result {
	return s.resolve(ctx, userID, videoID, false)
}

// ResolveAudio creates an audio-only session: neither the manifest nor its
// resource allowlist exposes a video stream. Shared health checks still probe
// both audio and video, independently of the viewer's selected playback mode.
func (s *Service) ResolveAudio(ctx context.Context, userID, videoID string) Result {
	return s.resolve(ctx, userID, videoID, true)
}

func (s *Service) resolve(ctx context.Context, userID, videoID string, audioOnly bool) Result {
	if userID == "" {
		return s.fallback("guest")
	}
	if s.config.ForceIframe {
		return s.fallback("disabled")
	}
	if s.base == nil {
		return s.fallback("unconfigured")
	}
	if !videoIDPattern.MatchString(videoID) {
		return s.fallback("invalid_video")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	s.mu.Lock()
	s.health.lastRequested = s.now()
	if s.resolveCount >= 8 || s.resolving[userID] >= 2 {
		s.mu.Unlock()
		return s.fallback("busy")
	}
	s.resolveCount++
	s.resolving[userID]++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.resolveCount--
		s.resolving[userID]--
		if s.resolving[userID] == 0 {
			delete(s.resolving, userID)
		}
		s.mu.Unlock()
	}()
	if !s.healthy(ctx) {
		if ctx.Err() != nil {
			return s.fallback("resolution_timeout")
		}
		return s.fallback("health_unavailable")
	}
	// Coalesce extraction, but create a different opaque session for each user.
	ch := s.flight.DoChan(videoID, func() (any, error) {
		// Detached, shared work remains bounded when a caller disconnects early.
		select {
		case s.extractions <- struct{}{}:
		default:
			return nil, ErrBusy
		}
		defer func() { <-s.extractions }()
		work, cancel := context.WithTimeout(s.ctx, 8*time.Second)
		defer cancel()
		return s.fetchManifest(work, videoID)
	})
	var r *resolved
	select {
	case <-ctx.Done():
		return s.fallback("resolution_timeout")
	case result := <-ch:
		if result.Err != nil {
			if errors.Is(result.Err, ErrInvalidManifest) {
				return s.fallback("invalid_manifest")
			}
			if errors.Is(result.Err, ErrBusy) {
				return s.fallback("busy")
			}
			if errors.Is(result.Err, ErrExpired) {
				return s.fallback("expired_urls")
			}
			return s.fallback("video_unavailable")
		}
		r = result.Val.(*resolved)
	}
	if !r.ExpiresAt.After(s.now().Add(2 * time.Minute)) {
		return s.fallback("expired_urls")
	}
	qualities := r.qualities()
	if audioOnly {
		r = r.audioOnly()
	}
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return s.fallback("internal_error")
	}
	id := base64.RawURLEncoding.EncodeToString(entropy[:])
	manifest, err := r.manifest(id)
	if err != nil {
		return s.fallback("invalid_manifest")
	}
	s.mu.Lock()
	var oldestID string
	var oldest time.Time
	count := 0
	for key, v := range s.sessions {
		if !v.ExpiresAt.After(s.now()) {
			delete(s.sessions, key)
			continue
		}
		if v.UserID == userID {
			count++
			if oldestID == "" || v.Created.Before(oldest) {
				oldestID = key
				oldest = v.Created
			}
		}
	}
	if count >= 8 {
		delete(s.sessions, oldestID)
	}
	if len(s.sessions) >= 2048 {
		s.mu.Unlock()
		return s.fallback("busy")
	}
	s.sessions[id] = &session{userID, manifest, r.Resources, r.ExpiresAt, s.now()}
	s.mu.Unlock()
	s.observe("resolution", "native")
	return Result{Mode: "native", Reason: "ready", ManifestURL: "/api/playback/" + id + "/manifest.mpd", ExpiresAt: r.ExpiresAt, Qualities: qualities, AudioOnly: audioOnly}
}

func (s *Service) fetchManifest(ctx context.Context, id string) (*resolved, error) {
	u := *s.base
	u.Path += "/api/manifest/dash/id/" + id
	query := url.Values{"local": {"true"}}
	if s.config.CompanionKey != "" {
		check, err := manifestCheck(s.config.CompanionKey, id, s.now())
		if err != nil {
			return nil, err
		}
		query.Set("check", check)
	}
	u.RawQuery = query.Encode()
	r, err := s.request(ctx, http.MethodGet, u.String(), "")
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, errors.New("Companion manifest unavailable")
	}
	const limit = 2 << 20
	data, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("manifest too large")
	}
	parsed, err := s.parseManifest(data)
	if err != nil && !errors.Is(err, ErrExpired) {
		return nil, errors.Join(ErrInvalidManifest, err)
	}
	return parsed, err
}

// Companion's SERVER_VERIFY_REQUESTS manifest authentication is AES-128-ECB
// with PKCS#7 padding of "unixSeconds|videoId", encoded as URL-safe base64.
// The Bearer header alone only authenticates its youtubei API.
func manifestCheck(secret, id string, now time.Time) (string, error) {
	if len(secret) != 16 {
		return "", errors.New("Companion secret must be 16 bytes")
	}
	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", err
	}
	plain := []byte(strconv.FormatInt(now.Unix(), 10) + "|" + id)
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	for i := 0; i < padding; i++ {
		plain = append(plain, byte(padding))
	}
	encrypted := make([]byte, len(plain))
	for i := 0; i < len(plain); i += aes.BlockSize {
		block.Encrypt(encrypted[i:i+aes.BlockSize], plain[i:i+aes.BlockSize])
	}
	return base64.URLEncoding.EncodeToString(encrypted), nil
}
func (s *Service) request(ctx context.Context, method, target, byteRange string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	if s.config.CompanionKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.CompanionKey)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	return s.client.Do(req)
}
func (s *Service) lookup(userID, id string) (*session, error) {
	v := s.sessions[id]
	if v == nil {
		return nil, ErrNotFound
	}
	if userID == "" || v.UserID != userID {
		return nil, ErrForbidden
	}
	if !v.ExpiresAt.After(s.now()) {
		delete(s.sessions, id)
		return nil, ErrExpired
	}
	return v, nil
}
func (s *Service) Manifest(userID, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.lookup(userID, id)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), v.Manifest...), nil
}

// OpenMedia returns a streaming response. The caller MUST close Body, including
// for HEAD and error HTTP statuses; Close cancels upstream work and frees a slot.
func (s *Service) OpenMedia(ctx context.Context, userID, id, resourceID, method, byteRange string) (*http.Response, error) {
	if method != http.MethodGet && method != http.MethodHead {
		return nil, ErrForbidden
	}
	if byteRange != "" && (!rangePattern.MatchString(byteRange) || len(byteRange) > 80) {
		return nil, ErrInvalidRange
	}
	s.mu.Lock()
	v, err := s.lookup(userID, id)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	index, err := strconv.Atoi(resourceID)
	if err != nil || index < 0 || index >= len(v.Resources) {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if s.mediaCount >= 64 || s.media[userID] >= 8 {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	target := v.Resources[index].URL
	s.mediaCount++
	s.media[userID]++
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	release := func() {
		cancel()
		s.mu.Lock()
		s.mediaCount--
		s.media[userID]--
		if s.media[userID] == 0 {
			delete(s.media, userID)
		}
		s.mu.Unlock()
	}
	r, err := s.request(ctx, method, target, byteRange)
	if err != nil {
		release()
		s.observe("media", "upstream_error")
		return nil, err
	}
	if r.StatusCode >= 400 {
		s.observe("media", "upstream_status")
	}
	if r.StatusCode >= 300 {
		// Upstream diagnostic pages can contain signed URLs or proxy secrets.
		// Preserve the status (and a 416 Content-Range), never its error body.
		r.Body.Close()
		const message = "Media unavailable\n"
		r.Body = io.NopCloser(strings.NewReader(message))
		r.ContentLength = int64(len(message))
		r.Header.Set("Content-Length", strconv.Itoa(len(message)))
		r.Header.Set("Content-Type", "text/plain; charset=utf-8")
		r.Header.Del("Location")
		r.Header.Del("Set-Cookie")
	}
	r.Body = &releasedBody{ReadCloser: r.Body, release: release}
	return r, nil
}

type releasedBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *releasedBody) Close() error { err := b.ReadCloser.Close(); b.once.Do(b.release); return err }
