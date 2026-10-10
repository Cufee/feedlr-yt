package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
)

func podcastValidationFixture(t *testing.T, hostURL string) (database.Client, podcastInput) {
	t.Helper()
	db := podcastLogicFixture(t)
	ctx := context.Background()
	v, err := db.GetVideoByID(ctx, "episode")
	if err != nil {
		t.Fatal(err)
	}
	v.MediaURL = null.StringFrom(hostURL)
	if err = db.UpsertVideos(ctx, v); err != nil {
		t.Fatal(err)
	}
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	return db, input
}

func TestPodcastByteSamplesDetectChangesWithSameValidators(t *testing.T) {
	data := bytes.Repeat([]byte("abcdef0123456789"), 32768)
	var requests atomic.Int32
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"unchanged"`)
		http.ServeContent(w, r, "audio.mp3", time.Unix(100, 0), bytes.NewReader(data))
	}))
	defer host.Close()
	ctx := context.Background()
	first, err := probePodcastAudio(ctx, host.URL)
	if err != nil || first.SampleHash == "" || first.Length != int64(len(data)) {
		t.Fatalf("probe %+v: %v", first, err)
	}
	path := filepath.Join(t.TempDir(), "audio")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	fileHash, err := hashPodcastFileSamples(path, int64(len(data)))
	if err != nil || fileHash != first.SampleHash {
		t.Fatalf("file/range sample hashes differ: %s != %s (%v)", fileHash, first.SampleHash, err)
	}
	// A host reusing validators and duration still cannot conceal a changed
	// sampled interior byte. This is separate from the complete audio hash.
	data[len(data)/2] ^= 1
	second, err := probePodcastAudio(ctx, host.URL)
	if err != nil || second.SampleHash == first.SampleHash || second.ETag != first.ETag || second.Length != first.Length {
		t.Fatalf("changed audio not detected: %+v, %v", second, err)
	}
	if requests.Load() != 12 {
		t.Fatalf("expected two bounded HEAD + five range checks, got %d", requests.Load())
	}
}

func TestPodcastValidationSharedAcrossListenersAndPolling(t *testing.T) {
	data := bytes.Repeat([]byte("audio"), 50000)
	var requests atomic.Int32
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"audio-v1"`)
		http.ServeContent(w, r, "audio.mp3", time.Unix(100, 0), bytes.NewReader(data))
	}))
	defer host.Close()
	db, input := podcastValidationFixture(t, host.URL)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			validated, err := revalidatePodcastInput(context.Background(), db, input, false)
			if err != nil || validated.validation.Fingerprint == "" || validated.transcriptKey == input.transcriptKey {
				t.Errorf("validation did not select rendition input: %+v %v", validated.validation, err)
			}
		}()
	}
	group.Wait()
	if requests.Load() != 6 {
		t.Fatalf("listeners did not share one check: %d requests", requests.Load())
	}
	for i := 0; i < 4; i++ {
		if _, err := GetPodcastSegmentStatus(context.Background(), db, "episode"); err != nil {
			t.Fatal(err)
		}
		if _, err := revalidatePodcastInput(context.Background(), db, input, false); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 6 {
		t.Fatal("polls or recent playback revalidation downloaded input")
	}
}

func TestPodcastExpiredValidationAndPublisherContentInvalidation(t *testing.T) {
	data := bytes.Repeat([]byte("audio"), 50000)
	publisher := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nNew publisher content"
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/text" {
			w.Write([]byte(publisher))
			return
		}
		w.Header().Set("ETag", `"audio-v1"`)
		http.ServeContent(w, r, "audio.mp3", time.Unix(100, 0), bytes.NewReader(data))
	}))
	defer host.Close()
	db, _ := podcastValidationFixture(t, host.URL)
	ctx := context.Background()
	if err := db.UpsertPodcastTranscript(ctx, database.PodcastTranscript{VideoID: "episode", URL: host.URL + "/text", MIMEType: "text/vtt"}); err != nil {
		t.Fatal(err)
	}
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	oldAudio, err := probePodcastAudio(ctx, host.URL)
	if err != nil {
		t.Fatal(err)
	}
	oldSources := podcastSourceInputs{Version: podcastInputVersion, Audio: oldAudio, Publisher: podcastSourceIdentity{URL: host.URL + "/text", ContentHash: podcastSHA256([]byte("old content"))}}
	encoded, _ := json.Marshal(oldSources)
	if err := db.SavePodcastSourceValidation(ctx, database.PodcastSourceValidation{VideoID: "episode", MetadataKey: input.metadataKey, Fingerprint: podcastSHA256(encoded), InputJSON: encoded, ValidatedAt: time.Now().Add(-2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	old, _ := loadPodcastInput(ctx, db, "episode")
	current, err := revalidatePodcastInput(ctx, db, old, false)
	if err != nil || current.transcriptKey == old.transcriptKey || current.jobKey == old.jobKey {
		t.Fatalf("publisher byte change did not invalidate input: %v", err)
	}
	if podcastValidationInputs(current).Publisher.ContentHash != podcastSHA256([]byte(publisher)) {
		t.Fatal("complete publisher content hash missing")
	}
}

func TestPodcastValidationHostCapabilitiesAndFailure(t *testing.T) {
	for _, mode := range []string{"no HEAD", "strong no ranges", "weak no ranges", "changed range", "changed no ranges", "slow"} {
		t.Run(mode, func(t *testing.T) {
			data := bytes.Repeat([]byte("audio"), 50000)
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "slow" {
					<-r.Context().Done()
					return
				}
				if mode == "no HEAD" && r.Method == http.MethodHead {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("ETag", `"v1"`)
				if mode == "weak no ranges" {
					w.Header().Set("ETag", `W/"v1"`)
				}
				if strings.Contains(mode, "no ranges") && r.Method == http.MethodGet {
					if mode == "changed no ranges" {
						w.Header().Set("ETag", `"v2"`)
					}
					w.Write(data)
					return
				}
				if mode == "changed range" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusPreconditionFailed)
					return
				}
				http.ServeContent(w, r, "audio.mp3", time.Unix(100, 0), bytes.NewReader(data))
			}))
			defer host.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_, err := probePodcastAudio(ctx, host.URL)
			wantSuccess := mode == "no HEAD" || mode == "strong no ranges"
			if (err == nil) != wantSuccess {
				t.Fatalf("probe: %v", err)
			}
		})
	}
}

func TestPodcastRenditionChangesBeforePaidTranscription(t *testing.T) {
	previous := openrouter.DefaultClient
	openrouter.DefaultClient = openrouter.New("unused-test-key", "test-model")
	t.Cleanup(func() { openrouter.DefaultClient = previous })
	data := bytes.Repeat([]byte("audio"), 50000)
	version := `"v1"`
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", version)
		http.ServeContent(w, r, "audio.mp3", time.Unix(100, 0), bytes.NewReader(data))
	}))
	defer host.Close()
	db, input := podcastValidationFixture(t, host.URL)
	ctx := context.Background()
	input, err := revalidatePodcastInput(ctx, db, input, false)
	if err != nil {
		t.Fatal(err)
	}
	version = `"v2"`
	_, err = generatePodcastTranscript(ctx, db, input)
	var failure *podcastProcessingError
	if !errors.As(err, &failure) || failure.code != "source_changed" {
		t.Fatalf("changed rendition should stop before FFmpeg/provider: %v", err)
	}
	current, err := loadPodcastInput(ctx, db, "episode")
	if err != nil || current.transcriptKey != input.transcriptKey {
		t.Fatalf("failed download must not silently replace accepted input: %v", err)
	}
	if _, claimed, err := db.ClaimPodcastProcessingJob(ctx, "unexpected", podcastJobLease); err != nil || claimed {
		t.Fatalf("rendition changes must not create a job/download chain: %v", err)
	}
	current, err = revalidatePodcastInput(ctx, db, current, true)
	if err != nil || current.transcriptKey == input.transcriptKey {
		t.Fatalf("explicit source-change retry must revalidate despite TTL: %v", err)
	}
}

func TestPodcastSourceURLNormalization(t *testing.T) {
	first := stablePodcastSourceURL("https://cdn.test/audio?exp=10&le=old&variant=one&X-Amz-Signature=first")
	second := stablePodcastSourceURL("https://cdn.test/audio?exp=20&le=new&variant=one&X-Amz-Signature=second")
	if first != second || first == stablePodcastSourceURL("https://cdn.test/audio?variant=two") {
		t.Fatal("signature changes should not hide actual variant changes")
	}
}
