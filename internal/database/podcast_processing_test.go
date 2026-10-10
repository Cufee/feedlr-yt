package database

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
)

// Separate connections to a real file exercise the same SQLite coordination
// used by separate application processes, rather than a shared Go mutex.
func podcastProcessingFixture(t *testing.T, count int, beforeMigration ...string) [2]*sqliteClient {
	t.Helper()
	path := filepath.Join(t.TempDir(), "podcasts.db")
	var clients [2]*sqliteClient
	for i := range clients {
		client, err := NewSQLiteClient(path)
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = client.(*sqliteClient)
		t.Cleanup(func() { _ = client.Close() })
	}
	migrations, err := filepath.Glob("migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, path := range migrations {
		if len(beforeMigration) > 0 && filepath.Base(path) == beforeMigration[0] {
			break
		}
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := clients[0].db.Exec(string(migration)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	ctx := context.Background()
	channel := &models.Channel{ID: "podcast"}
	if err := channel.Insert(ctx, clients[0].db, boil.Infer()); err != nil {
		t.Fatal(err)
	}
	for i := range count {
		video := &models.Video{ID: fmt.Sprintf("episode-%d", i), ChannelID: channel.ID, Type: "podcast_episode", MediaURL: null.StringFrom("https://example.com/audio.mp3")}
		if err := video.Insert(ctx, clients[0].db, boil.Infer()); err != nil {
			t.Fatal(err)
		}
	}
	return clients
}

func TestPodcastTranscriptContentCacheSeparatesMetadataVersions(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	v := PodcastTranscriptContent{
		VideoID: "episode-0", SourceKey: "source-v1", Source: "transcription", SourceURL: "https://example.com/audio.mp3",
		ContentHash: "content-v1", Model: "whisper", DurationMS: 90000,
		CuesJSON:  []byte(`[{"startMS":0,"endMS":90000,"text":"Complete episode"}]`),
		UsageJSON: []byte(`{"seconds":90}`),
		InputJSON: []byte(`{"metadata_key":"metadata-v1","audio_fingerprint":"audio-v1","source_key":"source-v1"}`),
	}
	if err := clients[0].SavePodcastTranscriptContent(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := clients[1].UpsertPodcastTranscript(ctx, PodcastTranscript{VideoID: v.VideoID, URL: "https://example.com/publisher.vtt", MIMEType: "text/vtt"}); err != nil {
		t.Fatal(err)
	}
	stored, err := clients[1].GetPodcastTranscriptContent(ctx, v.VideoID, v.SourceKey)
	if err != nil || stored.ContentHash != v.ContentHash || stored.Model != v.Model || stored.DurationMS != v.DurationMS || !bytes.Equal(stored.CuesJSON, v.CuesJSON) || !bytes.Equal(stored.UsageJSON, v.UsageJSON) || !bytes.Equal(stored.InputJSON, v.InputJSON) {
		t.Fatalf("complete transcript was not retained independently of publisher metadata: %+v, %v", stored, err)
	}
	if _, err := clients[1].GetPodcastTranscriptContent(ctx, v.VideoID, "source-v2"); !IsErrNotFound(err) {
		t.Fatalf("changed source reused stale transcript: %v", err)
	}
	v.SourceKey, v.ContentHash = "source-v2", "content-v2"
	if err := clients[1].SavePodcastTranscriptContent(ctx, v); err != nil {
		t.Fatal(err)
	}
	stored, err = clients[0].GetPodcastTranscriptContent(ctx, v.VideoID, "source-v1")
	if err != nil || stored.ContentHash != "content-v1" {
		t.Fatalf("new source overwrote old cache entry: %+v, %v", stored, err)
	}
	if _, err := clients[0].db.ExecContext(ctx, "DELETE FROM videos WHERE id = ?", v.VideoID); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[1].GetPodcastTranscriptContent(ctx, v.VideoID, v.SourceKey); !IsErrNotFound(err) {
		t.Fatalf("deleted video retained transcript content: %v", err)
	}
}

func TestPodcastSourceValidationPersistsNewestInputsByMetadataVersion(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	initial := PodcastSourceValidation{
		VideoID: "episode-0", MetadataKey: "metadata-v1", Fingerprint: "audio-v1",
		InputJSON:   []byte(`{"etag":"v1","content_length":123456,"resolved_path":"/episode.mp3"}`),
		ValidatedAt: time.Date(2026, 10, 10, 10, 10, 10, 123456789, time.FixedZone("offset", -3*60*60)),
	}
	if err := clients[0].SavePodcastSourceValidation(ctx, initial); err != nil {
		t.Fatal(err)
	}
	stored, err := clients[1].GetPodcastSourceValidation(ctx, initial.VideoID, initial.MetadataKey)
	if err != nil || stored.VideoID != initial.VideoID || stored.MetadataKey != initial.MetadataKey || stored.Fingerprint != initial.Fingerprint || !bytes.Equal(stored.InputJSON, initial.InputJSON) || !stored.ValidatedAt.Equal(initial.ValidatedAt) {
		t.Fatalf("source validation did not round trip: %+v, %v", stored, err)
	}
	if _, err := clients[1].GetPodcastSourceValidation(ctx, initial.VideoID, "metadata-v2"); !IsErrNotFound(err) {
		t.Fatalf("changed source metadata reused old validation: %v", err)
	}
	newest := initial
	newest.Fingerprint = "audio-v2"
	newest.InputJSON = []byte(`{"etag":"v2","content_length":234567,"resolved_path":"/episode.mp3"}`)
	newest.ValidatedAt = initial.ValidatedAt.Add(time.Minute)
	if err := clients[1].SavePodcastSourceValidation(ctx, newest); err != nil {
		t.Fatal(err)
	}
	if err := clients[0].SavePodcastSourceValidation(ctx, initial); err != nil {
		t.Fatal(err)
	}
	stored, err = clients[0].GetPodcastSourceValidation(ctx, initial.VideoID, initial.MetadataKey)
	if err != nil || stored.Fingerprint != newest.Fingerprint || !bytes.Equal(stored.InputJSON, newest.InputJSON) || !stored.ValidatedAt.Equal(newest.ValidatedAt) {
		t.Fatalf("older response replaced newer validation: %+v, %v", stored, err)
	}
	other := newest
	other.MetadataKey, other.Fingerprint, other.InputJSON = "metadata-v2", "other-audio", nil
	if err := clients[1].SavePodcastSourceValidation(ctx, other); err != nil {
		t.Fatal(err)
	}
	stored, err = clients[0].GetPodcastSourceValidation(ctx, other.VideoID, other.MetadataKey)
	if err != nil || stored.Fingerprint != "other-audio" || !bytes.Equal(stored.InputJSON, []byte("{}")) {
		t.Fatalf("new metadata version was not stored independently: %+v, %v", stored, err)
	}
	if _, err := clients[0].db.ExecContext(ctx, "DELETE FROM videos WHERE id = ?", initial.VideoID); err != nil {
		t.Fatal(err)
	}
	if _, err := clients[1].GetPodcastSourceValidation(ctx, initial.VideoID, initial.MetadataKey); !IsErrNotFound(err) {
		t.Fatalf("deleted episode retained validators: %v", err)
	}
}

func TestPodcastSegmentAnalysisInputsRemainBoundToCompletedSegments(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	analysis, acquired, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, "episode-0", "transcript-hash", "https://example.com/episode.mp3", "model", "prompt")
	if !acquired || err != nil {
		t.Fatalf("acquire analysis: %v, %v", acquired, err)
	}
	inputs := []byte(`{"video_id":"episode-0","metadata_key":"metadata","source_key":"resolved-source","audio_fingerprint":"audio-v1","transcript_hash":"transcript-hash","title_hash":"title","notes_hash":"notes","model":"model","prompt_version":"prompt"}`)
	if err := clients[0].SetPodcastSegmentAnalysisInputs(ctx, analysis.ID, inputs); err != nil {
		t.Fatal(err)
	}
	segments := []PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 2000, StartCue: 0, EndCue: 1, StartText: "Sponsor", EndText: "End", Reason: "Promotion"}}
	if err := clients[0].CompletePodcastSegmentAnalysis(ctx, analysis.ID, PodcastSegmentReady, "", segments); err != nil {
		t.Fatal(err)
	}
	stored, err := clients[1].GetPodcastSegmentAnalysisInputs(ctx, analysis.ID)
	if err != nil || !bytes.Equal(stored, inputs) {
		t.Fatalf("completed analysis lost provenance: %s, %v", stored, err)
	}
	completed, err := clients[1].GetPodcastSegmentAnalysis(ctx, analysis.VideoID, analysis.TranscriptHash, analysis.Model, analysis.PromptVersion)
	if err != nil || completed.ID != analysis.ID || completed.Status != PodcastSegmentReady || len(completed.Segments) != 1 || completed.Segments[0] != segments[0] {
		t.Fatalf("provenance update changed associated segments: %+v, %v", completed, err)
	}
	if err := clients[1].SetPodcastSegmentAnalysisInputs(ctx, "missing-analysis", inputs); !IsErrNotFound(err) {
		t.Fatalf("missing analysis silently accepted provenance: %v", err)
	}
	if _, err := clients[1].GetPodcastSegmentAnalysisInputs(ctx, "missing-analysis"); !IsErrNotFound(err) {
		t.Fatalf("missing analysis returned provenance: %v", err)
	}
}

func TestPodcastProvenanceMigrationPreservesExistingCachedRows(t *testing.T) {
	paths, err := filepath.Glob("migrations/*_add_podcast_source_provenance.sql")
	if err != nil || len(paths) != 1 {
		t.Fatalf("find provenance migration: %v, %v", paths, err)
	}
	clients := podcastProcessingFixture(t, 1, filepath.Base(paths[0]))
	ctx := context.Background()
	_, err = clients[0].db.ExecContext(ctx, `INSERT INTO podcast_transcript_contents
		(video_id, source_key, source, source_url, content_hash, model, duration_ms, cues_json, usage_json, updated_at_ms)
		VALUES ('episode-0', 'legacy-source', 'generated', 'https://example.com/episode.mp3', 'legacy-hash', 'whisper', 90000, ?, ?, ?)`, []byte(`[{"text":"Cached episode"}]`), []byte(`{"seconds":90}`), time.Now().UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	_, err = clients[0].db.ExecContext(ctx, `INSERT INTO podcast_segment_analyses
		(id, video_id, transcript_hash, transcript_url, model, prompt_version, status, created_at, updated_at)
		VALUES ('legacy-analysis', 'episode-0', 'legacy-hash', 'https://example.com/episode.mp3', 'model', 'prompt', 'ready', ?, ?)`, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients[0].db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	content, err := clients[1].GetPodcastTranscriptContent(ctx, "episode-0", "legacy-source")
	if err != nil || content.ContentHash != "legacy-hash" || content.DurationMS != 90000 || !bytes.Equal(content.InputJSON, []byte("{}")) || !bytes.Equal(content.UsageJSON, []byte(`{"seconds":90}`)) {
		t.Fatalf("migration changed existing transcript contents: %+v, %v", content, err)
	}
	inputs, err := clients[1].GetPodcastSegmentAnalysisInputs(ctx, "legacy-analysis")
	if err != nil || !bytes.Equal(inputs, []byte("{}")) {
		t.Fatalf("migration did not supply compatible existing analysis inputs: %s, %v", inputs, err)
	}
}

func TestPodcastProcessingConcurrentEnqueueSharesJobAndInvalidatesInputs(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	type result struct {
		job PodcastProcessingJob
		err error
	}
	results := make(chan result, 16)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			<-start
			job, err := clients[i%2].EnqueuePodcastProcessingJob(ctx, "episode-0", "source", "model", "prompt")
			results <- result{job, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var id string
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if id == "" {
			id = r.job.ID
		}
		if r.job.ID != id || r.job.Status != PodcastSegmentPending {
			t.Fatalf("enqueue created duplicate job: %+v", r.job)
		}
	}
	for _, input := range [][3]string{{"new-source", "model", "prompt"}, {"source", "new-model", "prompt"}, {"source", "model", "new-prompt"}} {
		job, err := clients[1].EnqueuePodcastProcessingJob(ctx, "episode-0", input[0], input[1], input[2])
		if err != nil || job.ID == id {
			t.Fatalf("changed inputs reused a job: %+v, %v", job, err)
		}
	}
}

func TestPodcastProcessingClaimLimitsAcrossConnections(t *testing.T) {
	clients := podcastProcessingFixture(t, 12)
	ctx := context.Background()
	for i := range 12 {
		if _, err := clients[0].EnqueuePodcastProcessingJob(ctx, fmt.Sprintf("episode-%d", i), "source", "model", "prompt"); err != nil {
			t.Fatal(err)
		}
	}
	type result struct {
		job     PodcastProcessingJob
		claimed bool
		err     error
	}
	results := make(chan result, 12)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			<-start
			job, claimed, err := clients[i%2].ClaimPodcastProcessingJob(ctx, fmt.Sprintf("worker-%d", i), time.Minute)
			results <- result{job, claimed, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	claimed := map[string]bool{}
	var active PodcastProcessingJob
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.claimed {
			if claimed[r.job.ID] || r.job.Status != PodcastSegmentRunning || r.job.Token == "" {
				t.Fatalf("invalid or repeated claim: %+v", r.job)
			}
			claimed[r.job.ID] = true
			active = r.job
		}
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed %d jobs, want global limit 2", len(claimed))
	}
	if ok, err := clients[1].FinishPodcastProcessingJob(ctx, active.ID, active.Token, PodcastSegmentReady, ""); !ok || err != nil {
		t.Fatalf("finish: %v, %v", ok, err)
	}
	if _, ok, err := clients[0].ClaimPodcastProcessingJob(ctx, "next-worker", time.Minute); !ok || err != nil {
		t.Fatalf("freed capacity was not available: %v, %v", ok, err)
	}
}

func TestPodcastProcessingExpiredLeaseRecoversAndFencesOldOwner(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	queued, err := clients[0].EnqueuePodcastProcessingJob(ctx, "episode-0", "source", "model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	job, ok, err := clients[0].ClaimPodcastProcessingJob(ctx, "old-owner", time.Minute)
	if !ok || err != nil || job.ID != queued.ID {
		t.Fatalf("initial claim: %+v, %v, %v", job, ok, err)
	}
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "analyzing", "transcript", "analysis", 12345); !ok || err != nil {
		t.Fatalf("set progress: %v, %v", ok, err)
	}
	if _, err := clients[1].db.ExecContext(ctx, "UPDATE podcast_processing_jobs SET lease_until_ms = 0 WHERE id = ?", job.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[0].RenewPodcastProcessingJob(ctx, job.ID, job.Token, time.Minute); ok || err != nil {
		t.Fatalf("expired owner renewed lease: %v, %v", ok, err)
	}
	if ok, err := clients[0].FinishPodcastProcessingJob(ctx, job.ID, job.Token, PodcastSegmentReady, ""); ok || err != nil {
		t.Fatalf("expired owner completed job: %v, %v", ok, err)
	}
	recovered, ok, err := clients[1].ClaimPodcastProcessingJob(ctx, "new-owner", time.Minute)
	if !ok || err != nil || recovered.ID != job.ID || recovered.TranscriptHash != "transcript" || recovered.DurationMS != 12345 {
		t.Fatalf("expired job was not recovered with progress: %+v, %v, %v", recovered, ok, err)
	}
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "stale", "wrong", "wrong", 1); ok || err != nil {
		t.Fatalf("stale owner overwrote progress: %v, %v", ok, err)
	}
	if ok, err := clients[0].FinishPodcastProcessingJob(ctx, job.ID, job.Token, PodcastSegmentFailed, "stale"); ok || err != nil {
		t.Fatalf("stale owner failed replacement job: %v, %v", ok, err)
	}
	if ok, err := clients[1].RenewPodcastProcessingJob(ctx, recovered.ID, recovered.Token, time.Minute); !ok || err != nil {
		t.Fatalf("replacement owner could not renew: %v, %v", ok, err)
	}
	if ok, err := clients[1].FinishPodcastProcessingJob(ctx, recovered.ID, recovered.Token, PodcastSegmentReady, ""); !ok || err != nil {
		t.Fatalf("replacement owner could not complete: %v, %v", ok, err)
	}
	finished, err := clients[0].GetPodcastProcessingJob(ctx, job.VideoID, job.SourceKey, job.Model, job.PromptVersion)
	if err != nil || finished.Status != PodcastSegmentReady || finished.TranscriptHash != "transcript" || finished.Token != "" {
		t.Fatalf("unexpected finished job: %+v, %v", finished, err)
	}
}

func TestPodcastProcessingExplicitRetryPreservesTranscriptCache(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	content := PodcastTranscriptContent{VideoID: "episode-0", SourceKey: "source", ContentHash: "hash", DurationMS: 10000, CuesJSON: []byte(`[{"text":"Cached"}]`)}
	if err := clients[0].SavePodcastTranscriptContent(ctx, content); err != nil {
		t.Fatal(err)
	}
	initial, err := clients[0].EnqueuePodcastProcessingJob(ctx, "episode-0", "source", "model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	for i, terminal := range []string{PodcastSegmentFailed, PodcastSegmentUnavailable, PodcastSegmentReady} {
		job, ok, err := clients[0].ClaimPodcastProcessingJob(ctx, fmt.Sprintf("owner-%d", i), time.Minute)
		if !ok || err != nil {
			t.Fatalf("claim retry: %v, %v", ok, err)
		}
		if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "analyzing", content.ContentHash, "analysis", content.DurationMS); !ok || err != nil {
			t.Fatalf("progress: %v, %v", ok, err)
		}
		concurrent, err := clients[1].EnqueuePodcastProcessingJob(ctx, job.VideoID, job.SourceKey, job.Model, job.PromptVersion)
		if err != nil || concurrent.Token != job.Token || concurrent.Status != PodcastSegmentRunning {
			t.Fatalf("enqueue disturbed active owner: %+v, %v", concurrent, err)
		}
		if ok, err := clients[0].FinishPodcastProcessingJob(ctx, job.ID, job.Token, terminal, "failure"); !ok || err != nil {
			t.Fatalf("finish: %v, %v", ok, err)
		}
		if _, ok, err := clients[1].ClaimPodcastProcessingJob(ctx, "automatic-retry", time.Minute); ok || err != nil {
			t.Fatalf("terminal job retried without enqueue: %v, %v", ok, err)
		}
		requeued, err := clients[1].EnqueuePodcastProcessingJob(ctx, job.VideoID, job.SourceKey, job.Model, job.PromptVersion)
		if err != nil || requeued.ID != initial.ID || requeued.TranscriptHash != content.ContentHash || requeued.DurationMS != content.DurationMS {
			t.Fatalf("retry discarded reusable progress: %+v, %v", requeued, err)
		}
		if terminal == PodcastSegmentReady {
			if requeued.Status != PodcastSegmentReady {
				t.Fatalf("enqueue repeated a completed job: %+v", requeued)
			}
		} else if requeued.Status != PodcastSegmentPending || requeued.Error != "" || requeued.Token != "" || requeued.AnalysisID != "" {
			t.Fatalf("explicit retry did not reset terminal job: %+v", requeued)
		}
		cached, err := clients[1].GetPodcastTranscriptContent(ctx, content.VideoID, content.SourceKey)
		if err != nil || !bytes.Equal(cached.CuesJSON, content.CuesJSON) {
			t.Fatalf("retry discarded transcript cache: %+v, %v", cached, err)
		}
	}
}

func TestPodcastTranscriptionSlotsEnforceGlobalLimitAndRecover(t *testing.T) {
	clients := podcastProcessingFixture(t, 0)
	ctx := context.Background()
	type result struct {
		token string
		ok    bool
		err   error
	}
	results := make(chan result, 16)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			<-start
			token := fmt.Sprintf("request-%d", i)
			ok, err := clients[i%2].AcquirePodcastTranscriptionSlot(ctx, token, time.Minute)
			results <- result{token, ok, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var active []string
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		if r.ok {
			active = append(active, r.token)
		}
	}
	if len(active) != 4 {
		t.Fatalf("acquired %d transcription slots, want global limit 4", len(active))
	}
	if ok, err := clients[1].AcquirePodcastTranscriptionSlot(ctx, active[0], time.Minute); !ok || err != nil {
		t.Fatalf("full pool blocked renewal of active slot: %v, %v", ok, err)
	}
	if err := clients[1].ReleasePodcastTranscriptionSlot(ctx, active[0]); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[0].AcquirePodcastTranscriptionSlot(ctx, "replacement", time.Minute); !ok || err != nil {
		t.Fatalf("released slot unavailable: %v, %v", ok, err)
	}
	if _, err := clients[0].db.ExecContext(ctx, "UPDATE podcast_transcription_slots SET lease_until_ms = 0 WHERE token = ?", active[1]); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].AcquirePodcastTranscriptionSlot(ctx, "recovered", time.Minute); !ok || err != nil {
		t.Fatalf("expired slot unavailable: %v, %v", ok, err)
	}
	if ok, err := clients[0].AcquirePodcastTranscriptionSlot(ctx, active[1], time.Minute); ok || err != nil {
		t.Fatalf("stale slot owner exceeded capacity: %v, %v", ok, err)
	}
}
