package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/openrouter"
)

type podcastChunkCacheTransport func(*http.Request) (*http.Response, error)

func (f podcastChunkCacheTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestPodcastTranscriptionChunkCacheRecovery(t *testing.T) {
	for _, executable := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(executable); err != nil {
			t.Skipf("chunk cache integration requires %s: %v", executable, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "silence.flac")
	if output, err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "anullsrc=r=16000:cl=mono", "-t", "610", "-sample_fmt", "s16", "-c:a", "flac", path).CombinedOutput(); err != nil {
		t.Fatalf("create local silence fixture: %v: %s", err, output)
	}
	audio, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var audioMu sync.RWMutex
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		audioMu.RLock()
		defer audioMu.RUnlock()
		w.Header().Set("Content-Type", "audio/flac")
		_, _ = w.Write(audio)
	}))
	defer host.Close()
	db := podcastLogicFixture(t)
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	input.video.MediaURL = null.StringFrom(host.URL)
	input.validation.InputJSON = []byte("{}")
	previousTransport, previousClient := http.DefaultTransport, openrouter.DefaultClient
	t.Cleanup(func() {
		http.DefaultTransport, openrouter.DefaultClient = previousTransport, previousClient
	})
	type reply struct {
		language, body string
		status         int
	}
	replies := []reply{
		{"", `{"text":"","language":"English","duration":600,"segments":[],"usage":{"seconds":600,"cost":0.002}}`, 200},
		{"en", `{"error":{"message":"terminal test failure"}}`, 400},
		{"en", `{"text":"","language":"English","duration":12}`, 200}, // Missing segments must not enter the cache.
		{"en", `{"text":"","duration":12,"segments":[],"usage":{"seconds":12}}`, 200},
		{"", `{"text":"","language":"English","duration":600,"segments":[]}`, 200},
		{"en", `{"text":"","language":"English","duration":12,"segments":[]}`, 200},
	}
	var calls int
	var uploads []string
	http.DefaultTransport = podcastChunkCacheTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme+"://"+r.URL.Host == host.URL {
			return previousTransport.RoundTrip(r)
		}
		if r.URL.Host != "openrouter.ai" || r.URL.Path != "/api/v1/audio/transcriptions" {
			t.Errorf("unexpected network destination: %s", r.URL)
			return &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		if calls >= len(replies) {
			t.Errorf("unexpected extra transcription call %d", calls+1)
			return &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
		reply := replies[calls]
		calls++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("multipart request: %v", err)
		} else {
			defer r.MultipartForm.RemoveAll()
			if r.Method != http.MethodPost || r.FormValue("model") != openrouter.TranscriptionModel || r.FormValue("language") != reply.language || r.FormValue("response_format") != "verbose_json" || r.FormValue("timestamp_granularities[]") != "segment" {
				t.Errorf("unexpected transcription fields: %v", r.MultipartForm.Value)
			}
			file, _, err := r.FormFile("file")
			if err != nil {
				t.Errorf("missing uploaded FLAC: %v", err)
			} else {
				data, err := io.ReadAll(file)
				file.Close()
				if err != nil || !strings.HasPrefix(string(data), "fLaC") {
					t.Errorf("invalid uploaded FLAC: %v", err)
				}
				uploads = append(uploads, podcastSHA256(data))
			}
		}
		return &http.Response{StatusCode: reply.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(reply.body))}, nil
	})
	openrouter.DefaultClient = openrouter.New("unused-local-test-key", "unrelated-sponsor-model")
	cacheKey := func(index int) string {
		return podcastSHA256(fmt.Appendf(nil, "%s:%s:chunk-v1:%d", input.transcriptKey, podcastSHA256(audio), index))
	}
	checkCached := func(index int, present bool) error {
		content, err := db.GetPodcastTranscriptContent(ctx, "episode", cacheKey(index))
		if !present {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("chunk %d should not be cached: %v", index, err)
			}
			return nil
		}
		var details podcastChunkInputs
		if err != nil || content.Source != "generated_chunk" || string(content.CuesJSON) != "[]" || json.Unmarshal(content.InputJSON, &details) != nil || details.Language != "en" || details.AudioHash != podcastSHA256(audio) {
			return fmt.Errorf("invalid durable chunk %d: %+v, %v", index, content, err)
		}
		return nil
	}
	for _, phase := range []struct {
		name       string
		wantCalls  int
		wantFailed bool
	}{
		{"terminal tail failure", 2, true},
		{"malformed tail response", 3, true},
		{"retry tail only", 4, false},
		{"all chunks cached", 4, false},
		{"changed download bytes", 6, false},
	} {
		t.Run(phase.name, func(t *testing.T) {
			if phase.name == "changed download bytes" {
				audioMu.Lock()
				audio = append(audio, 0) // Preserve decodable audio while changing the complete source hash.
				audioMu.Unlock()
			}
			var ready []int
			content, err := generatePodcastTranscriptChunks(ctx, db, input, func(chunk podcastTranscriptChunk) error {
				ready = append(ready, chunk.Index)
				return checkCached(chunk.Index, true) // Publication happens only after the raw chunk is durable.
			})
			if (err != nil) != phase.wantFailed || calls != phase.wantCalls {
				t.Fatalf("calls=%d want=%d, err=%v", calls, phase.wantCalls, err)
			}
			if err := checkCached(0, true); err != nil {
				t.Fatal(err)
			}
			if err := checkCached(1, !phase.wantFailed); err != nil {
				t.Fatal(err)
			}
			if phase.wantFailed {
				if len(ready) != 1 || ready[0] != 0 {
					t.Fatalf("failed tail published a chunk: %v", ready)
				}
			} else if content.DurationMS != 610000 || string(content.CuesJSON) != "[]" || len(ready) != 2 || ready[0] != 0 || ready[1] != 1 {
				t.Fatalf("complete cached silence: content=%+v, ready=%v", content, ready)
			}
		})
	}
	if len(uploads) != 6 || uploads[0] == uploads[1] || uploads[1] != uploads[2] || uploads[1] != uploads[3] || uploads[0] != uploads[4] || uploads[1] != uploads[5] {
		t.Fatalf("retry did not upload only the failed tail chunk: %v", uploads)
	}
}
