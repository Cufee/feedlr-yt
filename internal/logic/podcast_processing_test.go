package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

func podcastLogicFixture(t *testing.T) database.Client {
	t.Helper()
	path := filepath.Join(t.TempDir(), "podcast.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob("../database/migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("migrations: %v", err)
	}
	for _, path := range migrations {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = raw.Exec(string(data)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if err = (&models.Channel{ID: "podcast"}).Insert(context.Background(), raw, boil.Infer()); err != nil {
		t.Fatal(err)
	}
	if err = (&models.Video{ID: "episode", ChannelID: "podcast", Type: "podcast_episode", MediaURL: null.StringFrom("https://example.test/audio"), Duration: 3600}).Insert(context.Background(), raw, boil.Infer()); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	db, err := database.NewSQLiteClient(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// Unit processors inject generation. Mark their source freshly validated so
	// cached POST tests do not contact an actual audio host.
	input, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePodcastSourceValidation(context.Background(), database.PodcastSourceValidation{VideoID: "episode", MetadataKey: input.metadataKey, Fingerprint: "fixture", InputJSON: []byte("{}"), ValidatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return db
}

func fakeGeneratedContent() database.PodcastTranscriptContent {
	cues, _ := json.Marshal([]transcriptCue{{Index: 0, StartMS: 1000, EndMS: 3000, Text: "Welcome to the episode"}})
	return database.PodcastTranscriptContent{Source: "generated", SourceURL: "https://example.test/audio", Model: openrouter.TranscriptionModel, DurationMS: 3600000, CuesJSON: cues, UsageJSON: []byte(`{"chunks":[{"seconds":3600,"cost":0.03}]}`)}
}

func TestPodcastPublisherFallbackAndCaching(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	for _, test := range []struct {
		name, mime, body string
		status           int
		generated        bool
	}{
		{name: "usable publisher", mime: "text/vtt", body: "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nWelcome", status: 200},
		{name: "missing publisher", generated: true},
		{name: "untimed publisher", mime: "text/plain", body: "Welcome", status: 200, generated: true},
		{name: "fetch failure", mime: "text/vtt", status: 503, generated: true},
		{name: "malformed timing", mime: "text/vtt", body: "00:00:01.000 -->\nWelcome", status: 200, generated: true},
		{name: "out of order", mime: "text/vtt", body: "00:00:03.000 --> 00:00:04.000\nA\n\n00:00:01.000 --> 00:00:02.000\nB", status: 200, generated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := podcastLogicFixture(t)
			var downloads atomic.Int32
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				downloads.Add(1)
				w.WriteHeader(test.status)
				w.Write([]byte(test.body))
			}))
			defer host.Close()
			if test.mime != "" {
				if err := db.UpsertPodcastTranscript(context.Background(), database.PodcastTranscript{VideoID: "episode", URL: host.URL, MIMEType: test.mime}); err != nil {
					t.Fatal(err)
				}
			}
			input, err := loadPodcastInput(context.Background(), db, "episode")
			if err != nil {
				t.Fatal(err)
			}
			generated := 0
			p := podcastProcessor{db: db, generate: func(context.Context, database.Client, podcastInput) (database.PodcastTranscriptContent, error) {
				generated++
				return fakeGeneratedContent(), nil
			}}
			content, cues, err := p.prepare(context.Background(), input)
			if err != nil || len(cues) != 1 {
				t.Fatalf("prepare: %v, %+v", err, cues)
			}
			if (content.Source == "generated") != test.generated || generated != boolToInt(test.generated) {
				t.Fatalf("fallback source=%s generated=%d", content.Source, generated)
			}
			before := downloads.Load()
			cached, _, err := p.prepare(context.Background(), input)
			if err != nil || cached.ContentHash != content.ContentHash || downloads.Load() != before || generated != boolToInt(test.generated) {
				t.Fatalf("cache was not reused: %v", err)
			}
			for i := 0; i < 3; i++ {
				status, err := GetPodcastSegmentStatus(context.Background(), db, "episode")
				if err != nil || status.Status != "idle" {
					t.Fatalf("poll status: %+v, %v", status, err)
				}
			}
			if downloads.Load() != before {
				t.Fatal("polling fetched publisher transcript")
			}
		})
	}
}
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestPodcastTranscriptionFlagAndInputInvalidation(t *testing.T) {
	db := podcastLogicFixture(t)
	input, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	p := podcastProcessor{db: db, generate: func(context.Context, database.Client, podcastInput) (database.PodcastTranscriptContent, error) {
		t.Fatal("disabled flag generated transcript")
		return database.PodcastTranscriptContent{}, nil
	}}
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "false")
	_, _, err = p.prepare(context.Background(), input)
	var failure *podcastProcessingError
	if !errors.As(err, &failure) || failure.code != "transcription_disabled" {
		t.Fatalf("disabled: %v", err)
	}
	first := input
	v := *input.video
	v.Description = "Updated notes"
	if err = db.UpsertVideos(context.Background(), &v); err != nil {
		t.Fatal(err)
	}
	input, err = loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if input.transcriptKey != first.transcriptKey || input.jobKey == first.jobKey {
		t.Fatal("notes should invalidate scanning only")
	}
	v.MediaURL = null.StringFrom("https://example.test/changed")
	if err = db.UpsertVideos(context.Background(), &v); err != nil {
		t.Fatal(err)
	}
	changed, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if changed.transcriptKey == input.transcriptKey {
		t.Fatal("enclosure metadata did not invalidate transcript")
	}
	source := database.PodcastTranscript{VideoID: "episode", URL: "https://example.test/publisher", MIMEType: "text/vtt"}
	if err = db.UpsertPodcastTranscript(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	publisher, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if publisher.transcriptKey == changed.transcriptKey {
		t.Fatal("publisher metadata did not invalidate transcript")
	}
	if err = db.DeletePodcastTranscript(context.Background(), "episode"); err != nil {
		t.Fatal(err)
	}
	removed, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if removed.transcriptKey != changed.transcriptKey {
		t.Fatal("removed publisher remained an input")
	}
}

func TestPodcastScanRetryAndCrashReuseCompleteTranscript(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	previous := openrouter.DefaultClient
	openrouter.DefaultClient = nil
	t.Cleanup(func() { openrouter.DefaultClient = previous })
	db := podcastLogicFixture(t)
	ctx := context.Background()
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	generated, scans := 0, 0
	p := podcastProcessor{db: db, generate: func(context.Context, database.Client, podcastInput) (database.PodcastTranscriptContent, error) {
		generated++
		return fakeGeneratedContent(), nil
	}}
	p.scan = func(ctx context.Context, cues []transcriptCue, notes, title string) ([]database.PodcastSegment, error) {
		scans++
		content, err := db.GetPodcastTranscriptContent(ctx, "episode", input.transcriptKey)
		if err != nil || len(content.CuesJSON) == 0 {
			t.Fatal("scan started before complete transcript was persisted")
		}
		status, err := GetPodcastSegmentStatus(ctx, db, "episode")
		if err != nil || status.Phase != "scanning" || status.Source != "generated" || status.DurationMS != 3600000 {
			t.Fatalf("scanning status: %+v, %v", status, err)
		}
		if scans == 1 {
			return nil, errors.New("transient scan error")
		}
		return []database.PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 3000, StartText: "Welcome", EndText: "episode"}}, nil
	}
	job, err := db.EnqueuePodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate crash after transcript persistence but before analysis completion.
	old, claimed, err := db.ClaimPodcastProcessingJob(ctx, "old", 10*time.Millisecond)
	if err != nil || !claimed {
		t.Fatalf("claim: %v", err)
	}
	if _, _, err = p.prepare(ctx, input); err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	recovered, claimed, err := db.ClaimPodcastProcessingJob(ctx, "recovered", podcastJobLease)
	if err != nil || !claimed || recovered.ID != old.ID {
		t.Fatalf("recover: %v", err)
	}
	p.run(recovered)
	status, err := GetPodcastSegmentStatus(ctx, db, "episode")
	if err != nil || status.Status != "failed" || status.Error != "model_output_invalid" {
		t.Fatalf("scan failure: %+v, %v", status, err)
	}
	retry, err := db.EnqueuePodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if err != nil || retry.ID != job.ID {
		t.Fatalf("retry: %v", err)
	}
	claimedJob, claimed, err := db.ClaimPodcastProcessingJob(ctx, "retry", podcastJobLease)
	if err != nil || !claimed {
		t.Fatalf("retry claim: %v", err)
	}
	p.run(claimedJob)
	status, err = GetPodcastSegmentStatus(ctx, db, "episode")
	if err != nil || status.Status != "ready" || len(status.Segments) != 1 || generated != 1 || scans != 2 {
		t.Fatalf("ready: %+v, gen=%d scans=%d err=%v", status, generated, scans, err)
	}
	readyJob, err := db.GetPodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	inputs, err := db.GetPodcastSegmentAnalysisInputs(ctx, readyJob.AnalysisID)
	var provenance map[string]any
	if err != nil || json.Unmarshal(inputs, &provenance) != nil || provenance["transcript_source_key"] != input.transcriptKey || provenance["source_fingerprint"] != input.validation.Fingerprint || provenance["model"] != podcastModel() || provenance["prompt_version"] != podcastSegmentsPromptVersion {
		t.Fatalf("ready segments lost their generation inputs: %s, %v", inputs, err)
	}
	cached, err := EnsurePodcastSegmentAnalysis(ctx, db, "episode")
	if err != nil || cached.Status != "ready" || len(cached.Segments) != 1 {
		t.Fatalf("provider disabled lost completed cached scan: %+v %v", cached, err)
	}
	joined, err := db.EnqueuePodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if err != nil || joined.Status != "ready" {
		t.Fatalf("ready join: %+v %v", joined, err)
	}
	if _, claim, err := db.ClaimPodcastProcessingJob(ctx, "unneeded", podcastJobLease); err != nil || claim {
		t.Fatalf("completed scan claimed again: %v", err)
	}
}

func TestPodcastJobPreparationAndSilentTranscript(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	ctx := context.Background()
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	p := podcastProcessor{db: db, generate: func(ctx context.Context, _ database.Client, _ podcastInput) (database.PodcastTranscriptContent, error) {
		status, err := GetPodcastSegmentStatus(ctx, db, "episode")
		if err != nil || status.Phase != "preparing" {
			t.Fatalf("preparation phase: %+v %v", status, err)
		}
		content := fakeGeneratedContent()
		content.CuesJSON = []byte("[]")
		return content, nil
	}, scan: func(context.Context, []transcriptCue, string, string) ([]database.PodcastSegment, error) {
		t.Fatal("silence should not scan")
		return nil, nil
	}}
	if _, err = db.EnqueuePodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion); err != nil {
		t.Fatal(err)
	}
	job, claim, err := db.ClaimPodcastProcessingJob(ctx, "silence", podcastJobLease)
	if err != nil || !claim {
		t.Fatal(err)
	}
	p.run(job)
	status, err := GetPodcastSegmentStatus(ctx, db, "episode")
	if err != nil || status.Status != "ready" || len(status.Segments) != 0 {
		t.Fatalf("silence: %+v %v", status, err)
	}
}
