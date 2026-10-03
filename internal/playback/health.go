package playback

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var healthVideos = [2]string{"jNQXAC9IVRw", "aqz-KE-bpKQ"}

type healthState struct {
	lastRequested time.Time
	validUntil    time.Time
	nextRetry     time.Time
	open          bool
	failures      int
	interior      bool
	checking      chan struct{}
}

// healthy shares a bounded check independently of any one viewer's cancellation.
// Cached failure immediately falls back until the next retry window.
func (s *Service) healthy(ctx context.Context) bool {
	s.mu.Lock()
	now := s.now()
	if now.Before(s.health.validUntil) {
		s.mu.Unlock()
		return true
	}
	if s.health.open && now.Before(s.health.nextRetry) {
		s.mu.Unlock()
		return false
	}
	done := s.health.checking
	if done == nil {
		done = make(chan struct{})
		s.health.checking = done
		go s.checkHealth(done, s.health.open, s.health.interior)
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-done:
	}
	s.mu.Lock()
	ok := s.now().Before(s.health.validUntil)
	s.mu.Unlock()
	return ok
}
func (s *Service) checkHealth(done chan struct{}, recovering, interior bool) {
	// Each video gets half the initial budget so the second can distinguish a
	// restricted probe video from a Companion-wide failure.
	check := func(id string) bool {
		ctx, cancel := context.WithTimeout(s.ctx, 3500*time.Millisecond)
		defer cancel()
		return s.probe(ctx, id, interior) == nil
	}
	first := check(healthVideos[0])
	ok := first
	if recovering || !first {
		second := check(healthVideos[1])
		if recovering {
			ok = first && second
		} else {
			ok = second
		}
	}
	s.mu.Lock()
	s.health.interior = !interior
	if ok {
		s.health.open = false
		s.health.failures = 0
		s.health.validUntil = s.now().Add(2 * time.Minute)
		s.health.nextRetry = time.Time{}
	} else {
		s.health.open = true
		s.health.validUntil = time.Time{}
		s.health.failures++
		backoff := time.Minute << min(s.health.failures-1, 3)
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		s.health.nextRetry = s.now().Add(backoff)
	}
	s.health.checking = nil
	close(done)
	s.mu.Unlock()
	if ok {
		s.observe("health", "healthy")
	} else {
		s.observe("health", "unavailable")
	}
}
func (s *Service) healthLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			active := !s.health.lastRequested.IsZero() && s.now().Sub(s.health.lastRequested) < 10*time.Minute
			for id, v := range s.sessions {
				if !v.ExpiresAt.After(s.now()) {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
			if active {
				ctx, cancel := context.WithTimeout(s.ctx, 8*time.Second)
				s.healthy(ctx)
				cancel()
			}
		}
	}
}
func (s *Service) probe(ctx context.Context, id string, interior bool) error {
	r, err := s.fetchManifest(ctx, id)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, res := range r.Resources {
		if seen[res.Kind] {
			continue
		}
		seen[res.Kind] = true
		start := int64(0)
		if interior {
			start = 65536
		}
		response, err := s.request(ctx, http.MethodGet, res.URL, fmt.Sprintf("bytes=%d-%d", start, start+65535))
		if err != nil {
			return err
		}
		err = validateProbe(response, start)
		response.Body.Close()
		if err != nil {
			return err
		}
		if len(seen) == 2 {
			return nil
		}
	}
	return fmt.Errorf("missing probe streams")
}
func validateProbe(r *http.Response, start int64) error {
	if r.StatusCode != http.StatusPartialContent || r.ContentLength != 65536 {
		return fmt.Errorf("invalid partial response")
	}
	var first, last, total int64
	cr := r.Header.Get("Content-Range")
	if _, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &first, &last, &total); err != nil || first != start || last != start+65535 || total <= last || cr != fmt.Sprintf("bytes %d-%d/%d", first, last, total) {
		return fmt.Errorf("invalid content range")
	}
	mime := strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]
	if !strings.HasPrefix(mime, "audio/") && !strings.HasPrefix(mime, "video/") && mime != "application/octet-stream" {
		return fmt.Errorf("invalid media content type")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil || len(data) != 65536 {
		return fmt.Errorf("incomplete probe bytes")
	}
	if start == 0 {
		// ISO BMFF starts with a file type/segment box; WebM starts with EBML.
		mp4 := string(data[4:8]) == "ftyp" || string(data[4:8]) == "styp"
		webm := binary.BigEndian.Uint32(data[:4]) == 0x1a45dfa3
		if !mp4 && !webm {
			return fmt.Errorf("invalid media signature (%s)", strconv.Itoa(len(data)))
		}
	}
	return nil
}
