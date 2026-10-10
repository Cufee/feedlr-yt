package database

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"
)

func linkedPodcastAggregate(t *testing.T, c *sqliteClient) (PodcastProcessingJob, PodcastSegmentAnalysis) {
	t.Helper()
	ctx := context.Background()
	if _, err := c.EnqueuePodcastProcessingJob(ctx, "episode-0", "source", "model", "prompt"); err != nil {
		t.Fatal(err)
	}
	job, claimed, err := c.ClaimPodcastProcessingJob(ctx, "original-owner", time.Minute)
	if !claimed || err != nil {
		t.Fatalf("claim aggregate job: %v, %v", claimed, err)
	}
	analysis, _, err := c.AcquirePodcastSegmentAnalysis(ctx, job.VideoID, "aggregate-hash", "https://example.com/audio.mp3", job.Model, job.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := c.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", analysis.TranscriptHash, analysis.ID, 1200000); !ok || err != nil {
		t.Fatalf("link aggregate job: %v, %v", ok, err)
	}
	job.TranscriptHash, job.AnalysisID = analysis.TranscriptHash, analysis.ID
	return job, analysis
}

func assertPodcastAggregateSegments(t *testing.T, c *sqliteClient, analysis PodcastSegmentAnalysis, want []PodcastSegment) PodcastSegmentAnalysis {
	t.Helper()
	stored, err := c.GetPodcastSegmentAnalysis(context.Background(), analysis.VideoID, analysis.TranscriptHash, analysis.Model, analysis.PromptVersion)
	if err != nil || !slices.Equal(stored.Segments, want) {
		t.Fatalf("aggregate segments = %+v, want %+v; error = %v", stored.Segments, want, err)
	}
	return stored
}

func TestPodcastProgressPublicationFencesOwnershipAndPreservesRetrySegments(t *testing.T) {
	clients := podcastProcessingFixture(t, 2)
	ctx := context.Background()
	job, aggregate := linkedPodcastAggregate(t, clients[0])
	first := []PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 5000, StartCue: 0, EndCue: 1, Reason: "First core"}}
	canonical := append(slices.Clone(first), PodcastSegment{Category: "sponsor", StartMS: 601000, EndMS: 605000, StartCue: 20, EndCue: 21, Reason: "Second core"})
	if ok, err := clients[0].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, first); !ok || err != nil {
		t.Fatalf("publish initial confirmed segments: %v, %v", ok, err)
	}
	stored := assertPodcastAggregateSegments(t, clients[1], aggregate, first)
	if stored.Status != PodcastSegmentRunning || stored.CompletedAt.Valid {
		t.Fatalf("partial publication prematurely completed analysis: %+v", stored)
	}
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, "wrong-owner", aggregate.ID, nil); ok || err != nil {
		t.Fatalf("wrong owner erased confirmed segments: %v, %v", ok, err)
	}
	unlinked, _, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, job.VideoID, "unlinked-hash", "url", job.Model, job.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, unlinked.ID, canonical); ok || err != nil {
		t.Fatalf("owner published into an unlinked analysis: %v, %v", ok, err)
	}
	other, _, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, "episode-1", "other-hash", "url", job.Model, job.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", job.TranscriptHash, other.ID, 1200000); !ok || err != nil {
		t.Fatalf("set test link: %v, %v", ok, err)
	}
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, other.ID, canonical); ok || err != nil {
		t.Fatalf("owner published into another episode: %v, %v", ok, err)
	}
	assertPodcastAggregateSegments(t, clients[1], other, nil)
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", job.TranscriptHash, aggregate.ID, 1200000); !ok || err != nil {
		t.Fatalf("restore aggregate link: %v, %v", ok, err)
	}
	if _, err := clients[0].db.ExecContext(ctx, `UPDATE podcast_processing_jobs SET lease_until_ms = 0 WHERE id = ?`, job.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, canonical); ok || err != nil {
		t.Fatalf("expired owner published: %v, %v", ok, err)
	}
	if ok, err := clients[1].CompletePodcastProcessingAnalysis(ctx, job.ID, job.Token, aggregate.ID, canonical); ok || err != nil {
		t.Fatalf("expired owner completed aggregate: %v, %v", ok, err)
	}
	assertPodcastAggregateSegments(t, clients[1], aggregate, first)
	recovered, claimed, err := clients[1].ClaimPodcastProcessingJob(ctx, "replacement-owner", time.Minute)
	if !claimed || err != nil || recovered.ID != job.ID {
		t.Fatalf("recover owner: %+v, %v, %v", recovered, claimed, err)
	}
	if ok, err := clients[0].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, nil); ok || err != nil {
		t.Fatalf("stale owner published after recovery: %v, %v", ok, err)
	}
	if ok, err := clients[0].CompletePodcastProcessingAnalysis(ctx, job.ID, job.Token, aggregate.ID, nil); ok || err != nil {
		t.Fatalf("stale owner completed replacement aggregate: %v, %v", ok, err)
	}
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, recovered.ID, recovered.Token, aggregate.ID, canonical); !ok || err != nil {
		t.Fatalf("replacement owner could not publish canonical accumulation: %v, %v", ok, err)
	}
	assertPodcastAggregateSegments(t, clients[0], aggregate, canonical)
	if ok, err := clients[1].FinishPodcastProcessingJob(ctx, recovered.ID, recovered.Token, PodcastSegmentFailed, "later_chunk_failed"); !ok || err != nil {
		t.Fatalf("record later chunk failure: %v, %v", ok, err)
	}
	assertPodcastAggregateSegments(t, clients[0], aggregate, canonical)
	if _, err := clients[0].EnqueuePodcastProcessingJob(ctx, job.VideoID, job.SourceKey, job.Model, job.PromptVersion); err != nil {
		t.Fatal(err)
	}
	assertPodcastAggregateSegments(t, clients[1], aggregate, canonical)
	retry, claimed, err := clients[0].ClaimPodcastProcessingJob(ctx, "retry-owner", time.Minute)
	if !claimed || err != nil {
		t.Fatalf("claim explicit retry: %v, %v", claimed, err)
	}
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, retry.ID, retry.Token, "scanning", aggregate.TranscriptHash, aggregate.ID, 1200000); !ok || err != nil {
		t.Fatalf("relink retry aggregate: %v, %v", ok, err)
	}
	if ok, err := clients[0].PublishPodcastSegmentAnalysis(ctx, retry.ID, retry.Token, aggregate.ID, canonical); !ok || err != nil {
		t.Fatalf("republish retry without duplicating spans: %v, %v", ok, err)
	}
	if ok, err := clients[0].CompletePodcastProcessingAnalysis(ctx, retry.ID, retry.Token, aggregate.ID, canonical); !ok || err != nil {
		t.Fatalf("complete aggregate with live owner: %v, %v", ok, err)
	}
	stored = assertPodcastAggregateSegments(t, clients[1], aggregate, canonical)
	if stored.Status != PodcastSegmentReady || !stored.CompletedAt.Valid {
		t.Fatalf("final completion did not remain compatible: %+v", stored)
	}
}

func TestPodcastProgressPublicationRollsBackErrorsAndExpiryDuringReplacement(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	job, aggregate := linkedPodcastAggregate(t, clients[0])
	first := []PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 2000, Reason: "Confirmed"}}
	if ok, err := clients[0].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, first); !ok || err != nil {
		t.Fatalf("initial publication: %v, %v", ok, err)
	}
	_, err := clients[0].db.ExecContext(ctx, `CREATE TRIGGER reject_podcast_segment BEFORE INSERT ON podcast_episode_segments
		WHEN NEW.reason = 'reject' BEGIN SELECT RAISE(ABORT, 'test insertion failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	rejected := append(slices.Clone(first), PodcastSegment{Category: "sponsor", StartMS: 3000, EndMS: 4000, Reason: "reject"})
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, rejected); ok || err == nil {
		t.Fatalf("insertion failure was not rolled back: %v, %v", ok, err)
	}
	assertPodcastAggregateSegments(t, clients[0], aggregate, first)
	_, err = clients[0].db.ExecContext(ctx, `CREATE TRIGGER expire_podcast_owner AFTER INSERT ON podcast_episode_segments
		WHEN NEW.reason = 'expire' BEGIN UPDATE podcast_processing_jobs SET lease_until_ms = 0 WHERE analysis_id = NEW.analysis_id; END`)
	if err != nil {
		t.Fatal(err)
	}
	expired := append(slices.Clone(first), PodcastSegment{Category: "sponsor", StartMS: 3000, EndMS: 4000, Reason: "expire"})
	if ok, err := clients[1].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, expired); ok || err != nil {
		t.Fatalf("expiry during replacement was not fenced: %v, %v", ok, err)
	}
	if ok, err := clients[1].CompletePodcastProcessingAnalysis(ctx, job.ID, job.Token, aggregate.ID, expired); ok || err != nil {
		t.Fatalf("expiry during final completion was not fenced: %v, %v", ok, err)
	}
	stored := assertPodcastAggregateSegments(t, clients[0], aggregate, first)
	if stored.Status != PodcastSegmentRunning || stored.CompletedAt.Valid {
		t.Fatalf("expired completion leaked ready state: %+v", stored)
	}
	if ok, err := clients[0].RenewPodcastProcessingJob(ctx, job.ID, job.Token, time.Minute); !ok || err != nil {
		t.Fatalf("publication rollback changed original lease: %v, %v", ok, err)
	}
}

func TestPodcastProgressAggregateInputsFenceExpiredAndReplacedOwners(t *testing.T) {
	clients := podcastProcessingFixture(t, 2)
	ctx := context.Background()
	job, aggregate := linkedPodcastAggregate(t, clients[0])
	original := []byte(`{"owner":"original","confirmed_chunks":[0]}`)
	if ok, err := clients[0].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, aggregate.ID, original); !ok || err != nil {
		t.Fatalf("live owner could not write aggregate inputs: %v, %v", ok, err)
	}
	assertInputs := func(analysisID string, want []byte) {
		t.Helper()
		got, err := clients[1].GetPodcastSegmentAnalysisInputs(ctx, analysisID)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("aggregate inputs = %s, want %s; error = %v", got, want, err)
		}
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, job.ID, "wrong-owner", aggregate.ID, nil); ok || err != nil {
		t.Fatalf("wrong owner erased aggregate proofs: %v, %v", ok, err)
	}
	unlinked, _, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, job.VideoID, "unlinked-inputs", "url", job.Model, job.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, unlinked.ID, original); ok || err != nil {
		t.Fatalf("owner changed unlinked cache provenance: %v, %v", ok, err)
	}
	other, _, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, "episode-1", "other-inputs", "url", job.Model, job.PromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", job.TranscriptHash, other.ID, 1200000); !ok || err != nil {
		t.Fatalf("set cross-episode test link: %v, %v", ok, err)
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, other.ID, original); ok || err != nil {
		t.Fatalf("owner changed another episode's provenance: %v, %v", ok, err)
	}
	assertInputs(other.ID, []byte(`{}`))
	if ok, err := clients[0].UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", job.TranscriptHash, aggregate.ID, 1200000); !ok || err != nil {
		t.Fatalf("restore aggregate link: %v, %v", ok, err)
	}
	if _, err := clients[0].db.ExecContext(ctx, `UPDATE podcast_processing_jobs SET lease_until_ms = 0 WHERE id = ?`, job.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, aggregate.ID, nil); ok || err != nil {
		t.Fatalf("expired owner erased aggregate provenance: %v, %v", ok, err)
	}
	assertInputs(aggregate.ID, original)
	recovered, claimed, err := clients[1].ClaimPodcastProcessingJob(ctx, "proof-successor", time.Minute)
	if !claimed || err != nil || recovered.ID != job.ID {
		t.Fatalf("recover aggregate owner: %+v, %v, %v", recovered, claimed, err)
	}
	successor := []byte(`{"owner":"successor","confirmed_chunks":[0,1],"context_hash":"new-proof"}`)
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, recovered.ID, recovered.Token, aggregate.ID, successor); !ok || err != nil {
		t.Fatalf("successor could not write new proofs: %v, %v", ok, err)
	}
	if ok, err := clients[0].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, aggregate.ID, original); ok || err != nil {
		t.Fatalf("stale owner overwrote successor proofs: %v, %v", ok, err)
	}
	assertInputs(aggregate.ID, successor)
	if ok, err := clients[1].FinishPodcastProcessingJob(ctx, recovered.ID, recovered.Token, PodcastSegmentReady, ""); !ok || err != nil {
		t.Fatalf("finish successor job: %v, %v", ok, err)
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, recovered.ID, recovered.Token, aggregate.ID, nil); ok || err != nil {
		t.Fatalf("terminal job changed aggregate provenance: %v, %v", ok, err)
	}
	assertInputs(aggregate.ID, successor)
}

func TestPodcastProgressAggregateInputsRollBackExpiryDuringWrite(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	job, aggregate := linkedPodcastAggregate(t, clients[0])
	if ok, err := clients[0].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, aggregate.ID, nil); !ok || err != nil {
		t.Fatalf("default input JSON was not accepted: %v, %v", ok, err)
	}
	_, err := clients[0].db.ExecContext(ctx, `CREATE TRIGGER expire_podcast_input_owner AFTER UPDATE OF input_json ON podcast_segment_analyses
		WHEN CAST(NEW.input_json AS TEXT) LIKE '%expire%'
		BEGIN UPDATE podcast_processing_jobs SET lease_until_ms = 0 WHERE analysis_id = NEW.id; END`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := clients[1].SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, aggregate.ID, []byte(`{"expire":true}`)); ok || err != nil {
		t.Fatalf("lease expiry during provenance write was not fenced: %v, %v", ok, err)
	}
	got, err := clients[0].GetPodcastSegmentAnalysisInputs(ctx, aggregate.ID)
	if err != nil || !bytes.Equal(got, []byte(`{}`)) {
		t.Fatalf("expired input write escaped rollback: %s, %v", got, err)
	}
	if ok, err := clients[0].RenewPodcastProcessingJob(ctx, job.ID, job.Token, time.Minute); !ok || err != nil {
		t.Fatalf("rolled back input write changed original lease: %v, %v", ok, err)
	}
}

func TestPodcastChunkTranscriptCachesRetainSilentChunksAcrossRetry(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	chunks := []PodcastTranscriptContent{
		{VideoID: "episode-0", SourceKey: "parent-v1/chunk-v1/0", Source: "generated", Model: "whisper", ContentHash: "cue-hash", DurationMS: 600000,
			CuesJSON: []byte(`[{"startMS":0,"endMS":1000,"text":"Hello"}]`), InputJSON: []byte(`{"source_key":"parent-v1","audio_sha256":"audio-v1","chunk_index":0,"start_ms":0,"core_end_ms":600000,"end_ms":610000,"language":"en"}`)},
		{VideoID: "episode-0", SourceKey: "parent-v1/chunk-v1/1", Source: "generated", Model: "whisper", ContentHash: "silent-hash", DurationMS: 600000,
			CuesJSON: []byte(`[]`), InputJSON: []byte(`{"source_key":"parent-v1","audio_sha256":"audio-v1","chunk_index":1,"start_ms":600000,"core_end_ms":1200000,"end_ms":1200000,"language":"en"}`)},
	}
	for _, chunk := range chunks {
		if err := clients[0].SavePodcastTranscriptContent(ctx, chunk); err != nil {
			t.Fatal(err)
		}
	}
	job, _ := linkedPodcastAggregate(t, clients[0])
	if ok, err := clients[0].FinishPodcastProcessingJob(ctx, job.ID, job.Token, PodcastSegmentFailed, "third_chunk_failed"); !ok || err != nil {
		t.Fatalf("fail job: %v, %v", ok, err)
	}
	if _, err := clients[1].EnqueuePodcastProcessingJob(ctx, job.VideoID, job.SourceKey, job.Model, job.PromptVersion); err != nil {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		stored, err := clients[1].GetPodcastTranscriptContent(ctx, chunk.VideoID, chunk.SourceKey)
		if err != nil || stored.ContentHash != chunk.ContentHash || !bytes.Equal(stored.CuesJSON, chunk.CuesJSON) || !bytes.Equal(stored.InputJSON, chunk.InputJSON) {
			t.Fatalf("retry lost completed or silent chunk: %+v, %v", stored, err)
		}
	}
	for _, changed := range []string{"parent-v1", "parent-v2/chunk-v1/0", "parent-v1/chunk-v2/0", "parent-v1/chunk-v1/2"} {
		if _, err := clients[1].GetPodcastTranscriptContent(ctx, "episode-0", changed); !IsErrNotFound(err) {
			t.Fatalf("wrong source, normalization version, or chunk reused cache %q: %v", changed, err)
		}
	}
}

func TestPodcastCoreScanCachesInvalidateContextAndRemainSeparateFromAggregate(t *testing.T) {
	clients := podcastProcessingFixture(t, 1)
	ctx := context.Background()
	job, aggregate := linkedPodcastAggregate(t, clients[0])
	canonical := []PodcastSegment{{Category: "sponsor", StartMS: 1000, EndMS: 2000, Reason: "Episode aggregate"}}
	if ok, err := clients[0].PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, aggregate.ID, canonical); !ok || err != nil {
		t.Fatalf("publish aggregate: %v, %v", ok, err)
	}
	scanHash := "scan:source-v1:core-v1:context-v1:normalization-v1"
	core, _, err := clients[0].AcquirePodcastSegmentAnalysis(ctx, "episode-0", scanHash, "url", "model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	inputs := []byte(`{"kind":"chunk_scan","source_key":"source-v1","core_hash":"core-v1","context_hash":"context-v1","model":"model","prompt_version":"prompt"}`)
	if err := clients[0].SetPodcastSegmentAnalysisInputs(ctx, core.ID, inputs); err != nil {
		t.Fatal(err)
	}
	coreSegments := []PodcastSegment{{Category: "sponsor", StartMS: 601000, EndMS: 602000, Reason: "Core result"}}
	if err := clients[0].CompletePodcastSegmentAnalysis(ctx, core.ID, PodcastSegmentReady, "", coreSegments); err != nil {
		t.Fatal(err)
	}
	stored, err := clients[1].GetPodcastSegmentAnalysis(ctx, "episode-0", scanHash, "model", "prompt")
	if err != nil || stored.ID != core.ID || !slices.Equal(stored.Segments, coreSegments) {
		t.Fatalf("core scan cache did not round trip: %+v, %v", stored, err)
	}
	storedInputs, err := clients[1].GetPodcastSegmentAnalysisInputs(ctx, core.ID)
	if err != nil || !bytes.Equal(storedInputs, inputs) {
		t.Fatalf("core scan lost provenance: %s, %v", storedInputs, err)
	}
	for _, input := range [][3]string{
		{"scan:source-v2:core-v1:context-v1:normalization-v1", "model", "prompt"},
		{"scan:source-v1:core-v2:context-v1:normalization-v1", "model", "prompt"},
		{"scan:source-v1:core-v1:context-v2:normalization-v1", "model", "prompt"},
		{scanHash, "new-model", "prompt"},
		{scanHash, "model", "new-prompt"},
	} {
		if _, err := clients[1].GetPodcastSegmentAnalysis(ctx, "episode-0", input[0], input[1], input[2]); !IsErrNotFound(err) {
			t.Fatalf("changed scan inputs reused old cache %v: %v", input, err)
		}
	}
	latest, err := clients[1].GetLatestPodcastSegmentAnalysis(ctx, "episode-0")
	if err != nil || latest.ID != aggregate.ID || !slices.Equal(latest.Segments, canonical) {
		t.Fatalf("core cache replaced episode aggregate: %+v, %v", latest, err)
	}
}
