package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
)

func progressiveInvalidationInputs(t *testing.T, db database.Client) map[string]json.RawMessage {
	t.Helper()
	ctx := context.Background()
	input, err := loadPodcastInput(ctx, db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.GetPodcastProcessingJob(ctx, "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if err != nil {
		t.Fatal(err)
	}
	if job.AnalysisID == "" {
		return nil // A reclaimed job is linked to its aggregate by the worker.
	}
	data, err := db.GetPodcastSegmentAnalysisInputs(ctx, job.AnalysisID)
	var inputs map[string]json.RawMessage
	if err != nil || json.Unmarshal(data, &inputs) != nil {
		t.Fatalf("aggregate inputs: %s, %v", data, err)
	}
	return inputs
}

func awaitProgressiveInvalidationAttempt(t *testing.T, db database.Client, attempt int) map[string]json.RawMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		inputs := progressiveInvalidationInputs(t, db)
		var source struct{ Attempt int }
		if json.Unmarshal(inputs["source_inputs"], &source) == nil && source.Attempt == attempt {
			return inputs
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("retry did not durably process its first chunk")
	return nil
}

func TestProgressiveFullAudioChangeInvalidatesPartialProofs(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	for _, changed := range []bool{true, false} {
		t.Run(fmt.Sprintf("changed=%t", changed), func(t *testing.T) {
			db := podcastLogicFixture(t)
			first, firstContent := progressiveFixtureChunks(t, 3)
			retry, retryContent := progressiveFixtureChunks(t, 3)
			hash := "fixture"
			if changed {
				hash = "new-full-audio-outside-validation-samples"
			}
			var cues []transcriptCue
			for i := range retry {
				retry[i].InputJSON = fmt.Appendf(nil, `{"audio_sha256":%q,"attempt":2}`, hash)
				if changed {
					retry[i].Cues[0].Text = "Use Beta to save time on the new audio."
				}
				cues = append(cues, retry[i].Cues[0])
			}
			retryContent.CuesJSON, _ = json.Marshal(cues)
			retryContent.InputJSON = retry[0].InputJSON
			release, done := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			var attempts, scans, extracted atomic.Int32
			p := podcastProcessor{db: db, extractSponsors: func(context.Context, string) string {
				extracted.Add(1)
				return `["Acme"]`
			}, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
				if attempts.Add(1) == 1 {
					for _, chunk := range first[:2] {
						if err := ready(chunk); err != nil {
							return firstContent, err
						}
					}
					return firstContent, processingError("transcription_failed", errors.New("tail STT failed"))
				}
				if err := ready(retry[0]); err != nil {
					return retryContent, err
				}
				select {
				case <-release:
				case <-ctx.Done():
					return retryContent, ctx.Err()
				}
				for _, chunk := range retry[1:] {
					if err := ready(chunk); err != nil {
						return retryContent, err
					}
				}
				return retryContent, nil
			}, scanChunk: func(_ context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
				scans.Add(1)
				piece := progressiveOwnedPiece(input)
				if err := publish(piece); err != nil {
					return nil, err
				}
				return []podcastScanPiece{piece}, nil
			}}
			p.run(progressiveClaim(t, db))
			status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "failed" })
			before := progressiveInvalidationInputs(t, db)
			var priorProofs map[string]json.RawMessage
			if len(status.Segments) != 1 || status.Error != "transcription_failed" || scans.Load() != 1 || json.Unmarshal(before["generation_contexts"], &priorProofs) != nil || len(priorProofs) != 1 {
				t.Fatalf("first attempt did not retain a proven interval: %+v, inputs=%s", status, before)
			}
			job := progressiveClaim(t, db)
			go func() { p.run(job); close(done) }()
			t.Cleanup(func() {
				unblock()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("retry worker did not finish")
				}
			})
			blocked := awaitProgressiveInvalidationAttempt(t, db, 2)
			status, err := GetPodcastSegmentStatus(context.Background(), db, "episode")
			wantSegments := 1
			if changed {
				wantSegments = 0
				for _, key := range []string{"generation_contexts", "chunk_scans", "complete_transcript_inputs", "transcript_content_sha256"} {
					if _, exists := blocked[key]; exists {
						t.Errorf("new audio retained prior %s: %s", key, blocked[key])
					}
				}
			} else if string(blocked["generation_contexts"]) != string(before["generation_contexts"]) {
				t.Error("same audio discarded prior generation proofs")
			}
			if err != nil || status.Status != "running" || len(status.Segments) != wantSegments || scans.Load() != 1 || extracted.Load() != 1 {
				t.Fatalf("first chunk bypassed neighbor gate or invalidation: %+v, scans=%d notes=%d err=%v", status, scans.Load(), extracted.Load(), err)
			}
			unblock()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("retry did not finish")
			}
			status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
			wantScans := int32(3)
			if changed {
				wantScans++
			}
			if len(status.Segments) != 3 || scans.Load() != wantScans || extracted.Load() != 1 {
				t.Fatalf("retry reused wrong scan or notes: %+v, scans=%d notes=%d", status, scans.Load(), extracted.Load())
			}
			for i, segment := range status.Segments {
				if segment.StartText != retry[i].Cues[0].Text {
					t.Fatalf("old rendition interval survived retry: %+v", segment)
				}
			}
			final := progressiveInvalidationInputs(t, db)
			var fullHash string
			var complete struct {
				AudioHash string `json:"audio_sha256"`
			}
			if json.Unmarshal(final["transcript_content_sha256"], &fullHash) != nil || fullHash != podcastSHA256(retryContent.CuesJSON) || json.Unmarshal(final["complete_transcript_inputs"], &complete) != nil || complete.AudioHash != hash {
				t.Fatalf("missing new complete-transcript provenance: %s", final)
			}
		})
	}
}

func TestProgressiveSilentPartialRetryExtractsHintsWhenSpeechArrives(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 2)
	chunks[0].Cues = []transcriptCue{}
	chunks[1].Cues[0].Index = 0
	content.CuesJSON, _ = json.Marshal(chunks[1].Cues)
	var attempts, extracted, scans atomic.Int32
	release, done := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	p := podcastProcessor{db: db, extractSponsors: func(context.Context, string) string {
		extracted.Add(1)
		return `["Acme"]`
	}, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		attempt := attempts.Add(1)
		first := chunks[0]
		first.InputJSON = fmt.Appendf(nil, `{"audio_sha256":"fixture","attempt":%d}`, attempt)
		if err := ready(first); err != nil {
			return content, err
		}
		if attempt == 1 {
			return content, processingError("transcription_failed", errors.New("speech tail unavailable"))
		}
		select {
		case <-release:
		case <-ctx.Done():
			return content, ctx.Err()
		}
		if err := ready(chunks[1]); err != nil {
			return content, err
		}
		return content, nil
	}, scanChunk: func(_ context.Context, input podcastScanInput, notes, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		scans.Add(1)
		if notes != `["Acme"]` {
			return nil, fmt.Errorf("speech used placeholder hints: %s", notes)
		}
		piece := progressiveOwnedPiece(input)
		if err := publish(piece); err != nil {
			return nil, err
		}
		return []podcastScanPiece{piece}, nil
	}}
	p.run(progressiveClaim(t, db))
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "failed" })
	if len(status.Segments) != 0 || extracted.Load() != 0 || scans.Load() != 0 {
		t.Fatalf("partial silence called speech processing: %+v, notes=%d scans=%d", status, extracted.Load(), scans.Load())
	}
	if _, exists := progressiveInvalidationInputs(t, db)["explicit_sponsors"]; exists {
		t.Fatal("failed preparation persisted synthetic empty sponsor hints")
	}
	job := progressiveClaim(t, db)
	go func() { p.run(job); close(done) }()
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("speech retry worker did not finish")
		}
	})
	awaitProgressiveInvalidationAttempt(t, db, 2)
	if extracted.Load() != 0 || scans.Load() != 0 {
		t.Fatal("retry extracted sponsor hints before its first speech chunk")
	}
	unblock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("speech retry did not finish")
	}
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	if len(status.Segments) != 1 || extracted.Load() != 1 || scans.Load() != 1 || string(progressiveInvalidationInputs(t, db)["explicit_sponsors"]) != `["Acme"]` {
		t.Fatalf("recovered speech did not extract and persist hints once: %+v, notes=%d scans=%d", status, extracted.Load(), scans.Load())
	}
}
