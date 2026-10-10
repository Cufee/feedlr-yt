package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
)

func progressiveFixtureChunks(t *testing.T, count int) ([]podcastTranscriptChunk, database.PodcastTranscriptContent) {
	t.Helper()
	metadata, err := podcastAudioChunks(count * podcastChunkMS)
	if err != nil {
		t.Fatal(err)
	}
	chunks := make([]podcastTranscriptChunk, count)
	var cues []transcriptCue
	for i := range chunks {
		cue := transcriptCue{Index: i, StartMS: i*podcastChunkMS + 1000, EndMS: i*podcastChunkMS + 4000, Text: "Use Acme to save time on your work."}
		cues = append(cues, cue)
		chunks[i] = podcastTranscriptChunk{Index: i, DurationMS: count * podcastChunkMS, Chunks: metadata, Cues: []transcriptCue{cue}, Source: "generated", SourceURL: "https://example.test/audio", InputJSON: []byte(`{"audio_sha256":"fixture"}`)}
	}
	encoded, _ := json.Marshal(cues)
	return chunks, database.PodcastTranscriptContent{Source: "generated", SourceURL: "https://example.test/audio", Model: openrouter.TranscriptionModel, DurationMS: count * podcastChunkMS, CuesJSON: encoded, UsageJSON: []byte("{}"), InputJSON: []byte(`{"audio_sha256":"fixture"}`)}
}

func progressiveOwnedPiece(input podcastScanInput) podcastScanPiece {
	var segments []database.PodcastSegment
	for i, cue := range input.Cues {
		if input.owns(cue) {
			segments = append(segments, database.PodcastSegment{Category: "sponsor", Brand: "Acme", Reason: "Dedicated commercial", StartMS: cue.StartMS, EndMS: cue.EndMS, StartCue: i, EndCue: i, StartText: cue.Text, EndText: cue.Text})
		}
	}
	return podcastScanPiece{Segments: segments, FirstChunk: input.CoreIndex, LastChunk: input.CoreIndex}
}

func progressiveClaim(t *testing.T, db database.Client) database.PodcastProcessingJob {
	t.Helper()
	input, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.EnqueuePodcastProcessingJob(context.Background(), "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion); err != nil {
		t.Fatal(err)
	}
	job, ok, err := db.ClaimPodcastProcessingJob(context.Background(), "test", podcastJobLease)
	if err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	return job
}

func awaitProgressiveStatus(t *testing.T, db database.Client, condition func(PodcastSegmentStatus) bool) PodcastSegmentStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, err := GetPodcastSegmentStatus(context.Background(), db, "episode")
		if err != nil {
			t.Fatal(err)
		}
		if condition(status) {
			return status
		}
		time.Sleep(time.Millisecond)
	}
	status, _ := GetPodcastSegmentStatus(context.Background(), db, "episode")
	t.Fatalf("status condition timed out: %+v", status)
	return status
}

func TestProgressivePublishesBeforeFullTranscriptWithNeighborGates(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 4)
	release := make(chan struct{})
	var firstPairReady atomic.Bool
	var lastReady atomic.Bool
	var calls atomic.Int32
	p := podcastProcessor{db: db, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		for _, i := range []int{0, 1} {
			if err := ready(chunks[i]); err != nil {
				return content, err
			}
		}
		firstPairReady.Store(true)
		select {
		case <-release:
		case <-ctx.Done():
			return content, ctx.Err()
		}
		// Out-of-order completion must still require the missing neighbor.
		if err := ready(chunks[3]); err != nil {
			return content, err
		}
		lastReady.Store(true)
		if err := ready(chunks[2]); err != nil {
			return content, err
		}
		return content, nil
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		calls.Add(1)
		if !firstPairReady.Load() {
			t.Error("scan before first neighbor was ready")
		}
		if input.CoreIndex != 0 && !lastReady.Load() {
			t.Error("scan without required later neighbor")
		}
		piece := progressiveOwnedPiece(input)
		if err := publish(piece); err != nil {
			return nil, err
		}
		return []podcastScanPiece{piece}, nil
	}}
	job := progressiveClaim(t, db)
	done := make(chan struct{})
	go func() { p.run(job); close(done) }()
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return len(s.Segments) > 0 })
	if status.Status != "running" || status.Phase != "scanning" || status.Source != "generated" || status.DurationMS != 4*podcastChunkMS || calls.Load() != 1 {
		t.Fatalf("early result: %+v calls=%d", status, calls.Load())
	}
	input, _ := loadPodcastInput(context.Background(), db, "episode")
	if _, err := db.GetPodcastTranscriptContent(context.Background(), "episode", input.transcriptKey); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("full transcript already saved: %v", err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("processing did not finish")
	}
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	if len(status.Segments) != 4 || calls.Load() != 4 {
		t.Fatalf("final %+v calls=%d", status, calls.Load())
	}
}

func TestProgressiveFailureRetainsSegmentsAndRetriesOnlyFailedCore(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 3)
	var generated atomic.Int32
	var extracted atomic.Int32
	var mu sync.Mutex
	counts := map[int]int{}
	p := podcastProcessor{db: db, extractSponsors: func(context.Context, string) string {
		extracted.Add(1)
		return `["Acme"]`
	}, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		generated.Add(1)
		for _, chunk := range chunks {
			if err := ready(chunk); err != nil {
				return content, err
			}
		}
		return content, nil
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		mu.Lock()
		counts[input.CoreIndex]++
		count := counts[input.CoreIndex]
		mu.Unlock()
		if input.CoreIndex == 1 && count == 1 {
			return nil, errors.New("invalid provider output")
		}
		piece := progressiveOwnedPiece(input)
		if err := publish(piece); err != nil {
			return nil, err
		}
		return []podcastScanPiece{piece}, nil
	}}
	p.run(progressiveClaim(t, db))
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "failed" })
	if len(status.Segments) != 2 || status.Error != "model_output_invalid" {
		t.Fatalf("failed snapshot %+v", status)
	}
	p.run(progressiveClaim(t, db))
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	mu.Lock()
	defer mu.Unlock()
	if len(status.Segments) != 3 || generated.Load() != 1 || extracted.Load() != 1 || counts[0] != 1 || counts[1] != 2 || counts[2] != 1 {
		t.Fatalf("retry %+v generation=%d notes=%d calls=%v", status, generated.Load(), extracted.Load(), counts)
	}
	input, _ := loadPodcastInput(context.Background(), db, "episode")
	job, _ := db.GetPodcastProcessingJob(context.Background(), "episode", input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	data, err := db.GetPodcastSegmentAnalysisInputs(context.Background(), job.AnalysisID)
	var inputs map[string]any
	if err != nil || json.Unmarshal(data, &inputs) != nil || inputs["generation_contexts"] == nil || inputs["transcript_content_sha256"] == nil {
		t.Fatalf("missing durable provenance %s %v", data, err)
	}
}

func TestProgressiveCrossCoreAdWaitsForBothScansAndPreservesGap(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 2)
	chunks[0].Cues[0].StartMS = podcastChunkMS - 10000
	chunks[0].Cues[0].EndMS = podcastChunkMS - 100
	chunks[1].Cues[0].StartMS = podcastChunkMS + 100
	chunks[1].Cues[0].EndMS = podcastChunkMS + 10000
	content.CuesJSON, _ = json.Marshal([]transcriptCue{chunks[0].Cues[0], chunks[1].Cues[0]})
	release := make(chan struct{})
	firstFinished := make(chan struct{})
	p := podcastProcessor{db: db, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		for _, c := range chunks {
			if err := ready(c); err != nil {
				return content, err
			}
		}
		return content, nil
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		if input.CoreIndex == 1 {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		piece := progressiveOwnedPiece(input)
		piece.FirstChunk = 0
		piece.LastChunk = 1
		if err := publish(piece); err != nil {
			return nil, err
		}
		if input.CoreIndex == 0 {
			close(firstFinished)
		}
		return []podcastScanPiece{piece}, nil
	}}
	done := make(chan struct{})
	go func() { p.run(progressiveClaim(t, db)); close(done) }()
	select {
	case <-firstFinished:
	case <-time.After(3 * time.Second):
		t.Fatal("first scan stalled")
	}
	status, _ := GetPodcastSegmentStatus(context.Background(), db, "episode")
	if status.Status != "running" || len(status.Segments) != 0 {
		t.Fatalf("cross-core ad published too early %+v", status)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("job stalled")
	}
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	if len(status.Segments) != 2 || status.Segments[0].EndMS != podcastChunkMS-100 || status.Segments[1].StartMS != podcastChunkMS+100 {
		t.Fatalf("audible gap lost %+v", status)
	}
}

func TestProgressiveExpandsBeyondNeighborsAndReusesExpandedCache(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 5)
	var mu sync.Mutex
	counts := map[int]int{}
	p := podcastProcessor{db: db, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		for _, c := range chunks {
			if err := ready(c); err != nil {
				return content, err
			}
		}
		return content, nil
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		mu.Lock()
		counts[input.CoreIndex]++
		n := counts[input.CoreIndex]
		mu.Unlock()
		if input.CoreIndex == 1 && input.LastChunk == 2 {
			return nil, &podcastContextDeferred{Right: true}
		}
		if input.CoreIndex == 2 && n == 1 {
			return nil, errors.New("retry this core")
		}
		if input.CoreIndex == 1 && input.LastChunk != 3 {
			t.Errorf("expanded context ends at %d", input.LastChunk)
		}
		piece := progressiveOwnedPiece(input)
		if err := publish(piece); err != nil {
			return nil, err
		}
		return []podcastScanPiece{piece}, nil
	}}
	p.run(progressiveClaim(t, db))
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "failed" })
	if len(status.Segments) != 4 {
		t.Fatalf("partial %+v", status)
	}
	p.run(progressiveClaim(t, db))
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	mu.Lock()
	defer mu.Unlock()
	if len(status.Segments) != 5 || counts[0] != 1 || counts[1] != 2 || counts[2] != 2 || counts[3] != 1 || counts[4] != 1 {
		t.Fatalf("expanded retry %+v calls=%v", status, counts)
	}
}

func TestProgressiveLaterTranscriptionFailureKeepsConfirmedSegments(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 4)
	p := podcastProcessor{db: db, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		for _, c := range chunks[:2] {
			if err := ready(c); err != nil {
				return content, err
			}
		}
		return content, processingError("transcription_failed", errors.New("later chunk failed"))
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		piece := progressiveOwnedPiece(input)
		if err := publish(piece); err != nil {
			return nil, err
		}
		return []podcastScanPiece{piece}, nil
	}}
	p.run(progressiveClaim(t, db))
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "failed" })
	if len(status.Segments) != 1 || status.Error != "transcription_failed" || status.Source != "generated" {
		t.Fatalf("failed preparation discarded confirmations %+v", status)
	}
}

func TestProgressiveContextHashesAreSensitiveToInputs(t *testing.T) {
	db := podcastLogicFixture(t)
	source, _ := loadPodcastInput(context.Background(), db, "episode")
	chunks, _ := progressiveFixtureChunks(t, 3)
	available := []bool{true, true, true}
	input, ok, err := makePodcastScanInput(chunks, available, 1, 0, 2)
	if err != nil || !ok {
		t.Fatal(err)
	}
	key, _, contextHash := podcastScanHashes(input, source)
	input.Cues[0].Text = "A changed neighboring ad rendition."
	changedKey, _, changedContext := podcastScanHashes(input, source)
	if key != changedKey || contextHash == changedContext {
		t.Fatal("neighbor change should invalidate context but preserve core identity")
	}
	input.Cues[1].Text = "Changed current chunk."
	changedCore, _, _ := podcastScanHashes(input, source)
	if changedCore == key {
		t.Fatal("changed current chunk reused scan identity")
	}
}

func TestProgressiveNegativeScansAreCachedAndContextChangesInvalidate(t *testing.T) {
	db := podcastLogicFixture(t)
	source, _ := loadPodcastInput(context.Background(), db, "episode")
	chunks, _ := progressiveFixtureChunks(t, 3)
	input, _, _ := makePodcastScanInput(chunks, []bool{true, true, true}, 1, 0, 2)
	var calls int
	p := podcastProcessor{db: db, scanChunk: func(context.Context, podcastScanInput, string, string, func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		calls++
		return []podcastScanPiece{}, nil
	}}
	run := func(input podcastScanInput) {
		t.Helper()
		events := make(chan podcastScanEvent, 2)
		p.scanProgressiveCore(context.Background(), source, input, "[]", events)
		event := <-events
		if event.Err != nil || len(event.Pieces) != 0 {
			t.Fatalf("negative scan %+v", event)
		}
	}
	run(input)
	run(input)
	if calls != 1 {
		t.Fatalf("negative cache repeated provider work: %d", calls)
	}
	input.Cues[0].Text = "Changed neighboring commercial context."
	run(input)
	run(input)
	if calls != 2 {
		t.Fatalf("changed context reused old scan: %d", calls)
	}
	source.video.Description = "New sponsor notes"
	run(input)
	if calls != 3 {
		t.Fatalf("changed notes reused scan: %d", calls)
	}
}

func TestProgressiveSilentCoreMakesNoScanCalls(t *testing.T) {
	db := podcastLogicFixture(t)
	source, _ := loadPodcastInput(context.Background(), db, "episode")
	chunks, _ := progressiveFixtureChunks(t, 3)
	chunks[1].Cues = []transcriptCue{}
	input, _, err := makePodcastScanInput(chunks, []bool{true, true, true}, 1, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	p := podcastProcessor{db: db, scanChunk: func(context.Context, podcastScanInput, string, string, func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		t.Fatal("silent core called provider even though neighboring chunks contain speech")
		return nil, nil
	}}
	for range 2 {
		events := make(chan podcastScanEvent, 2)
		p.scanProgressiveCore(context.Background(), source, input, "[]", events)
		if event := <-events; event.Err != nil || len(event.Pieces) > 0 {
			t.Fatalf("silent cache %+v", event)
		}
	}
}

func TestProgressiveLiveAndCompleteCachedContextsMatchAtOverlapEdges(t *testing.T) {
	metadata, _ := podcastAudioChunks(4 * podcastChunkMS)
	results := []openrouter.TranscriptionResult{
		{Segments: []openrouter.TranscriptionSegment{{Start: 599, End: 600, Text: "Previous edge."}}},
		{Segments: []openrouter.TranscriptionSegment{{Start: .5, End: 3, Text: "Overlapping boundary phrase."}, {Start: 4, End: 6, Text: "Retained context."}}},
		{Segments: []openrouter.TranscriptionSegment{{Start: 3, End: 5, Text: "Current core."}}},
		{Segments: []openrouter.TranscriptionSegment{{Start: 3, End: 5, Text: "Next context."}}},
	}
	live := make([]podcastTranscriptChunk, len(metadata))
	for i := range live {
		live[i] = podcastTranscriptChunk{Index: i, Chunks: metadata, Result: results[i], DurationMS: 4 * podcastChunkMS, Source: "generated"}
	}
	cues, err := mergePodcastTranscription(metadata, results, 4*podcastChunkMS)
	if err != nil {
		t.Fatal(err)
	}
	var cached []podcastTranscriptChunk
	content := database.PodcastTranscriptContent{Source: "generated", DurationMS: 4 * podcastChunkMS}
	if err := emitPodcastTranscriptChunks(content, cues, func(c podcastTranscriptChunk) error { cached = append(cached, c); return nil }); err != nil {
		t.Fatal(err)
	}
	a, _, err := makePodcastScanInput(live, []bool{true, true, true, true}, 2, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := makePodcastScanInput(cached, []bool{true, true, true, true}, 2, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a.Cues)
	bj, _ := json.Marshal(b.Cues)
	if string(aj) != string(bj) {
		t.Fatalf("live/cache drift\nlive %s\ncache %s", aj, bj)
	}
}

func TestProgressiveSilentEpisodeMakesNoSponsorProviderCalls(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 2)
	for i := range chunks {
		chunks[i].Cues = []transcriptCue{}
	}
	content.CuesJSON = []byte("[]")
	p := podcastProcessor{db: db, extractSponsors: func(context.Context, string) string {
		t.Error("silent episode extracted sponsor hints")
		return "[]"
	}, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		for _, chunk := range chunks {
			if err := ready(chunk); err != nil {
				return content, err
			}
		}
		return content, nil
	}, scanChunk: func(context.Context, podcastScanInput, string, string, func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		t.Error("silent episode called detector")
		return nil, nil
	}}
	p.run(progressiveClaim(t, db))
	status := awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	if len(status.Segments) > 0 {
		t.Fatalf("silent episode has segments: %+v", status)
	}
}

func TestProgressiveCancellationJoinsPreparationAndNotes(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 2)
	job := progressiveClaim(t, db)
	source, err := loadPodcastInput(context.Background(), db, "episode")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notesStarted := make(chan struct{})
	cleaning := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	cleanup := func(ctx context.Context) {
		<-ctx.Done()
		cleaning <- struct{}{}
		<-release
	}
	p := podcastProcessor{db: db, extractSponsors: func(ctx context.Context, _ string) string {
		close(notesStarted)
		cleanup(ctx)
		return "[]"
	}, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		if err := ready(chunks[0]); err != nil {
			return content, err
		}
		cleanup(ctx)
		return content, ctx.Err()
	}}
	done := make(chan error, 1)
	go func() { done <- p.processProgressive(ctx, job, source) }()
	select {
	case <-notesStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("notes did not start with first speech chunk")
	}
	cancel()
	for range 2 {
		select {
		case <-cleaning:
		case <-time.After(3 * time.Second):
			t.Fatal("producer did not receive cancellation")
		}
	}
	select {
	case err := <-done:
		t.Fatalf("job returned before producer cleanup: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("job failed to join completed cleanup")
	}
}

func TestProgressiveIntroWaitsForCoreAndMergedDurationGuard(t *testing.T) {
	t.Setenv("PODCAST_TRANSCRIPTION_ENABLED", "true")
	db := podcastLogicFixture(t)
	chunks, content := progressiveFixtureChunks(t, 1)
	chunks[0].Cues = []transcriptCue{
		{Index: 0, StartMS: 1000, EndMS: 41000, Text: "First cold open passage."},
		{Index: 1, StartMS: 41000, EndMS: 81000, Text: "Second cold open passage."},
	}
	content.CuesJSON, _ = json.Marshal(chunks[0].Cues)
	release := make(chan struct{})
	published := make(chan struct{})
	p := podcastProcessor{db: db, extractSponsors: func(context.Context, string) string { return "[]" }, generateChunks: func(ctx context.Context, _ database.Client, _ podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
		return content, ready(chunks[0])
	}, scanChunk: func(ctx context.Context, input podcastScanInput, _, _ string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
		var pieces []podcastScanPiece
		for i, cue := range input.Cues {
			piece := podcastScanPiece{FirstChunk: 0, LastChunk: 0, AwaitCore: true, Segments: []database.PodcastSegment{{Category: "intro", Reason: "Opening passage", StartMS: cue.StartMS, EndMS: cue.EndMS, StartCue: i, EndCue: i, StartText: cue.Text, EndText: cue.Text}}}
			if err := publish(piece); err != nil {
				return nil, err
			}
			pieces = append(pieces, piece)
			if i == 0 {
				close(published)
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
		return pieces, nil
	}}
	job := progressiveClaim(t, db)
	done := make(chan struct{})
	go func() { p.run(job); close(done) }()
	select {
	case <-published:
	case <-time.After(3 * time.Second):
		t.Fatal("intro not discovered")
	}
	// A manifest is written after processing a piece event, so this checks
	// publication after the event loop has accepted the first intro proposal.
	deadline := time.Now().Add(3 * time.Second)
	for {
		current, _ := db.GetPodcastProcessingJob(context.Background(), job.VideoID, job.SourceKey, job.Model, job.PromptVersion)
		data, _ := db.GetPodcastSegmentAnalysisInputs(context.Background(), current.AnalysisID)
		var inputs map[string]any
		_ = json.Unmarshal(data, &inputs)
		if inputs["chunk_scans"] != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("piece event never accepted")
		}
		time.Sleep(time.Millisecond)
	}
	status, _ := GetPodcastSegmentStatus(context.Background(), db, "episode")
	if status.Status != "running" || len(status.Segments) != 0 {
		t.Fatalf("intro published before all proposals were known: %+v", status)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("intro core did not complete")
	}
	status = awaitProgressiveStatus(t, db, func(s PodcastSegmentStatus) bool { return s.Status == "ready" })
	if len(status.Segments) != 0 {
		t.Fatalf("adjacent intros bypassed merged duration guard: %+v", status)
	}
}

func TestProgressivePieceValidationRequiresCompleteDependencies(t *testing.T) {
	chunks, _ := progressiveFixtureChunks(t, 3)
	chunks[1].Cues[0].StartMS = podcastChunkMS
	chunks[1].Cues[0].EndMS = 2 * podcastChunkMS
	input, _, err := makePodcastScanInput(chunks, []bool{true, true, true}, 1, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	piece := progressiveOwnedPiece(input)
	if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err == nil {
		t.Fatal("boundary-touching piece did not require both adjacent core scans")
	}
	piece.FirstChunk, piece.LastChunk = 0, 2
	if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err != nil {
		t.Fatal(err)
	}
	piece.Segments[0].Category = "intro"
	if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err == nil {
		t.Fatal("intro did not require completed core")
	}
	piece.AwaitCore = true
	if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err != nil {
		t.Fatal(err)
	}
	piece.FirstChunk = 2
	if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err == nil {
		t.Fatal("cache dependency range excluded its own core")
	}
}
