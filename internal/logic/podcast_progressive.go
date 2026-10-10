package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
)

const podcastProgressiveVersion = "podcast-progressive-v1"
const podcastCoreWorkers = 2

type podcastChunkInputs struct {
	AudioHash string `json:"audio_sha256"`
	SourceKey string `json:"transcript_source_key"`
	Index     int    `json:"chunk_index"`
	StartMS   int    `json:"start_ms"`
	EndMS     int    `json:"end_ms"`
	Language  string `json:"language"`
}

// Every event represents a durably cached transcription, including silence.
type podcastTranscriptChunk struct {
	Index, DurationMS int
	Chunks            []podcastAudioChunk
	Result            openrouter.TranscriptionResult
	Cues              []transcriptCue // Already stitched complete/publisher transcripts.
	Source, SourceURL string
	InputJSON         []byte
}

func podcastResultFromCues(chunk podcastAudioChunk, cues []transcriptCue, language string, usage json.RawMessage) openrouter.TranscriptionResult {
	r := openrouter.TranscriptionResult{Language: language, Usage: usage, Segments: make([]openrouter.TranscriptionSegment, 0, len(cues))}
	for _, cue := range cues {
		r.Segments = append(r.Segments, openrouter.TranscriptionSegment{Start: float64(cue.StartMS-chunk.StartMS) / 1000, End: float64(cue.EndMS-chunk.StartMS) / 1000, Text: cue.Text})
	}
	return r
}

func emitPodcastTranscriptChunks(content database.PodcastTranscriptContent, cues []transcriptCue, ready func(podcastTranscriptChunk) error) error {
	duration := content.DurationMS
	for _, cue := range cues {
		duration = max(duration, cue.EndMS)
	}
	chunks, err := podcastAudioChunks(duration)
	if err != nil {
		return err
	}
	owned := make([][]transcriptCue, len(chunks))
	for i := range owned {
		owned[i] = []transcriptCue{}
	}
	for _, cue := range cues {
		i := min(len(chunks)-1, (cue.StartMS+(cue.EndMS-cue.StartMS)/2)/podcastChunkMS)
		owned[i] = append(owned[i], cue)
	}
	for i := range chunks {
		if err := ready(podcastTranscriptChunk{Index: i, DurationMS: duration, Chunks: chunks, Cues: owned[i], Source: content.Source, SourceURL: content.SourceURL, InputJSON: content.InputJSON}); err != nil {
			return err
		}
	}
	return nil
}

type podcastScanInput struct {
	Cues                                                      []transcriptCue
	CoreIndex, FirstChunk, LastChunk, TotalChunks, DurationMS int
	Calls                                                     *atomic.Int32
	Limiter                                                   chan struct{}
}

func (s podcastScanInput) owns(c transcriptCue) bool {
	midpoint := c.StartMS + (c.EndMS-c.StartMS)/2
	return midpoint >= s.CoreIndex*podcastChunkMS && midpoint < min((s.CoreIndex+1)*podcastChunkMS, s.DurationMS)
}

type podcastContextDeferred struct {
	Left, Right           bool
	FirstChunk, LastChunk *int
}

func (*podcastContextDeferred) Error() string { return "waiting for additional transcript context" }

// Pieces retain their independently confirmed bounds. Cross-core pieces wait
// until all affected core scans settle before entering the canonical snapshot.
type podcastScanPiece struct {
	Segments   []database.PodcastSegment `json:"segments"`
	FirstChunk int                       `json:"first_chunk"`
	LastChunk  int                       `json:"last_chunk"`
	AwaitCore  bool                      `json:"await_core,omitempty"`
}

type podcastChunkScanner func(context.Context, podcastScanInput, string, string, func(podcastScanPiece) error) ([]podcastScanPiece, error)

func inferPodcastChunkSegments(ctx context.Context, input podcastScanInput, notes, title string, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
	if openrouter.DefaultClient == nil {
		return nil, errors.New("segment provider unavailable")
	}
	d := &segmentDetector{client: openrouter.DefaultClient, cues: input.Cues, title: title, notes: notes, progressive: &input, sharedCalls: input.Calls, limiter: input.Limiter}
	return detectPodcastChunk(ctx, d, input, publish)
}

func detectPodcastChunk(ctx context.Context, d *segmentDetector, input podcastScanInput, publish func(podcastScanPiece) error) ([]podcastScanPiece, error) {
	if len(input.Cues) == 0 {
		return []podcastScanPiece{}, nil
	}
	var mu sync.Mutex
	var pieces []podcastScanPiece
	d.confirmed = func(segments []database.PodcastSegment, span groundedSegment) error {
		var owned []database.PodcastSegment
		for _, segment := range segments {
			a, b := segment.StartCue, segment.EndCue
			for a <= b && !input.owns(input.Cues[a]) {
				a++
			}
			for b >= a && !input.owns(input.Cues[b]) {
				b--
			}
			if a > b {
				continue
			}
			first, last := input.Cues[a], input.Cues[b]
			segment.StartMS, segment.EndMS = first.StartMS, last.EndMS
			segment.StartCue, segment.EndCue = a, b
			segment.StartText, segment.EndText = first.Text, last.Text
			owned = append(owned, segment)
		}
		if len(owned) == 0 {
			return nil
		}
		piece := podcastScanPiece{Segments: owned,
			FirstChunk: min(input.TotalChunks-1, max(0, input.Cues[span.start.cue].StartMS-1)/podcastChunkMS),
			LastChunk:  min(input.TotalChunks-1, input.Cues[span.end.cue].EndMS/podcastChunkMS),
			AwaitCore:  span.category == "intro"}
		mu.Lock()
		pieces = append(pieces, piece)
		mu.Unlock()
		if publish != nil {
			return publish(piece)
		}
		return nil
	}
	_, err := d.detect(ctx)
	return pieces, err
}

type podcastCoreCache struct {
	Version, SourceKey, CoreHash, ContextHash string
	CoreIndex, FirstChunk, LastChunk          int
	Pieces                                    []podcastScanPiece
}

type podcastCoreState struct {
	First, Last               int
	Running, Complete, Failed bool
	Pieces                    []podcastScanPiece
	CacheKey, ContextHash     string
}

type podcastScanEvent struct {
	Index  int
	Piece  *podcastScanPiece
	Pieces []podcastScanPiece
	Err    error
}

func makePodcastScanInput(chunks []podcastTranscriptChunk, available []bool, core, first, last int) (podcastScanInput, bool, error) {
	for i := first; i <= last; i++ {
		if !available[i] {
			return podcastScanInput{}, false, nil
		}
	}
	input := podcastScanInput{CoreIndex: core, FirstChunk: first, LastChunk: last, TotalChunks: len(chunks), DurationMS: chunks[core].DurationMS}
	if chunks[core].Cues != nil {
		for i := first; i <= last; i++ {
			input.Cues = append(input.Cues, chunks[i].Cues...)
		}
	} else {
		metadata := chunks[core].Chunks[first : last+1]
		results := make([]openrouter.TranscriptionResult, last-first+1)
		for i := first; i <= last; i++ {
			results[i-first] = chunks[i].Result
		}
		cues, err := mergePodcastTranscription(metadata, results, input.DurationMS)
		if err != nil {
			return input, false, err
		}
		input.Cues = cues
	}
	if first > 0 && chunks[core].Source == "generated" {
		// Normalizing an artificial left edge against the missing previous
		// chunk can change overlap cues. Omit this tiny context edge in both
		// live and complete-cache buffers, so successful scans remain reusable.
		trimmed := input.Cues[:0]
		for _, cue := range input.Cues {
			if cue.StartMS >= first*podcastChunkMS+podcastOverlapMS {
				trimmed = append(trimmed, cue)
			}
		}
		input.Cues = trimmed
	}
	for i := range input.Cues {
		input.Cues[i].Index = i
	}
	if err := validateTranscriptCues(input.Cues, input.DurationMS, true); err != nil {
		return input, false, err
	}
	return input, true, nil
}

func podcastScanHashes(input podcastScanInput, source podcastInput) (key, coreHash, contextHash string) {
	var core []transcriptCue
	for _, cue := range input.Cues {
		if input.owns(cue) {
			cue.Index = len(core)
			core = append(core, cue)
		}
	}
	encoded, _ := json.Marshal(core)
	coreHash = podcastSHA256(encoded)
	context, _ := json.Marshal(input.Cues)
	contextHash = podcastSHA256(context)
	key = hashTranscript(encoded, source.transcriptKey, fmt.Sprintf("%s:core:%d", podcastProgressiveVersion, input.CoreIndex), source.video.Description, source.video.Title)
	return
}

func validatePodcastPieces(pieces []podcastScanPiece, input podcastScanInput) error {
	for _, piece := range pieces {
		if piece.FirstChunk < 0 || piece.LastChunk >= input.TotalChunks || piece.FirstChunk > input.CoreIndex || piece.LastChunk < input.CoreIndex {
			return errors.New("invalid cached chunk dependencies")
		}
		for _, s := range piece.Segments {
			if !validCategory(s.Category) || strings.TrimSpace(s.Reason) == "" || s.StartMS < 0 || s.EndMS <= s.StartMS || s.EndMS > input.DurationMS || s.StartCue < 0 || s.EndCue >= len(input.Cues) || s.EndCue < s.StartCue {
				return errors.New("invalid cached chunk segment")
			}
			if s.StartMS != input.Cues[s.StartCue].StartMS || s.EndMS != input.Cues[s.EndCue].EndMS || s.StartText != input.Cues[s.StartCue].Text || s.EndText != input.Cues[s.EndCue].Text || !input.owns(input.Cues[s.StartCue]) || !input.owns(input.Cues[s.EndCue]) {
				return errors.New("cached chunk segment does not match transcript")
			}
			if piece.FirstChunk > max(0, s.StartMS-1)/podcastChunkMS || piece.LastChunk < min(input.TotalChunks-1, s.EndMS/podcastChunkMS) || (s.Category == "intro" && !piece.AwaitCore) {
				return errors.New("cached chunk segment is missing dependencies")
			}
		}
	}
	return nil
}

func (p *podcastProcessor) scanProgressiveCore(ctx context.Context, source podcastInput, input podcastScanInput, notes string, events chan<- podcastScanEvent) {
	key, coreHash, contextHash := podcastScanHashes(input, source)
	send := func(event podcastScanEvent) error {
		select {
		case events <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	result, err := p.db.GetPodcastSegmentAnalysis(ctx, source.video.ID, key, podcastModel(), podcastSegmentsPromptVersion)
	if err == nil && result.Status == database.PodcastSegmentReady {
		data, loadErr := p.db.GetPodcastSegmentAnalysisInputs(ctx, result.ID)
		var cache podcastCoreCache
		if loadErr != nil {
			_ = send(podcastScanEvent{Index: input.CoreIndex, Err: loadErr})
			return
		}
		if json.Unmarshal(data, &cache) == nil && cache.Version == podcastProgressiveVersion && cache.SourceKey == source.transcriptKey && cache.CoreIndex == input.CoreIndex && cache.CoreHash == coreHash && cache.FirstChunk >= 0 && cache.LastChunk < input.TotalChunks && cache.FirstChunk <= input.CoreIndex && cache.LastChunk >= input.CoreIndex {
			if cache.FirstChunk < input.FirstChunk || cache.LastChunk > input.LastChunk {
				_ = send(podcastScanEvent{Index: input.CoreIndex, Err: &podcastContextDeferred{FirstChunk: &cache.FirstChunk, LastChunk: &cache.LastChunk}})
				return
			}
			if cache.ContextHash == contextHash {
				if err = validatePodcastPieces(cache.Pieces, input); err == nil {
					_ = send(podcastScanEvent{Index: input.CoreIndex, Pieces: cache.Pieces})
					return
				}
				_ = send(podcastScanEvent{Index: input.CoreIndex, Err: processingError("cached_scan_invalid", err)})
				return
			}
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_ = send(podcastScanEvent{Index: input.CoreIndex, Err: err})
		return
	}
	var pieces []podcastScanPiece
	hasSpeech := false
	for _, cue := range input.Cues {
		hasSpeech = hasSpeech || input.owns(cue)
	}
	if hasSpeech {
		pieces, err = p.scanChunk(ctx, input, notes, source.video.Title, func(piece podcastScanPiece) error {
			if err := validatePodcastPieces([]podcastScanPiece{piece}, input); err != nil {
				return err
			}
			return send(podcastScanEvent{Index: input.CoreIndex, Piece: &piece})
		})
	} else {
		err = nil
		pieces = []podcastScanPiece{}
	}
	if err == nil {
		err = validatePodcastPieces(pieces, input)
	}
	if err == nil {
		analysis, _, e := p.db.AcquirePodcastSegmentAnalysis(ctx, source.video.ID, key, source.video.MediaURL.String, podcastModel(), podcastSegmentsPromptVersion)
		if e == nil {
			data, _ := json.Marshal(podcastCoreCache{Version: podcastProgressiveVersion, SourceKey: source.transcriptKey, CoreHash: coreHash, ContextHash: contextHash, CoreIndex: input.CoreIndex, FirstChunk: input.FirstChunk, LastChunk: input.LastChunk, Pieces: pieces})
			e = p.db.SetPodcastSegmentAnalysisInputs(ctx, analysis.ID, data)
			if e == nil {
				var segments []database.PodcastSegment
				for _, piece := range pieces {
					segments = append(segments, piece.Segments...)
				}
				e = p.db.CompletePodcastSegmentAnalysis(ctx, analysis.ID, database.PodcastSegmentReady, "", mergePodcastSegments(segments))
			}
		}
		err = e
	}
	_ = send(podcastScanEvent{Index: input.CoreIndex, Pieces: pieces, Err: err})
}

// One event loop owns scheduling and publication. STT and core scans run in
// parallel; model requests still share one episode-wide budget and semaphore.
func (p *podcastProcessor) processProgressive(parent context.Context, job database.PodcastProcessingJob, source podcastInput) (resultErr error) {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	aggregateHash := hashTranscript(nil, source.transcriptKey, podcastProgressiveVersion, source.video.Description, source.video.Title)
	analysis, _, err := p.db.AcquirePodcastSegmentAnalysis(ctx, job.VideoID, aggregateHash, source.video.MediaURL.String, job.Model, job.PromptVersion)
	if err != nil {
		return err
	}
	ok, err := p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "preparing", aggregateHash, analysis.ID, job.DurationMS)
	if err != nil || !ok {
		return processingError("lease_lost", err)
	}
	if analysis.Status == database.PodcastSegmentReady {
		return nil
	}
	inputs := map[string]any{"version": podcastProgressiveVersion, "transcript_source_key": source.transcriptKey, "scan_input_hash": aggregateHash, "source_fingerprint": source.validation.Fingerprint, "title_sha256": podcastSHA256([]byte(source.video.Title)), "notes_sha256": podcastSHA256([]byte(source.video.Description)), "model": job.Model, "prompt_version": job.PromptVersion, "prompts_sha256": podcastSHA256([]byte(segmentDiscoveryPrompt + "\x00" + segmentConfirmationPrompt + "\x00" + segmentBoundaryReviewPrompt + "\x00" + segmentResponsePrompt + "\x00" + sponsorExtractionPrompt))}
	previousInputs, err := p.db.GetPodcastSegmentAnalysisInputs(ctx, analysis.ID)
	if err != nil {
		return err
	}
	var previous map[string]any
	var previousSource struct {
		Source string `json:"transcript_source"`
		Inputs struct {
			AudioHash string `json:"audio_sha256"`
		} `json:"source_inputs"`
	}
	_ = json.Unmarshal(previousInputs, &previousSource)
	if json.Unmarshal(previousInputs, &previous) == nil {
		for _, key := range []string{"transcript_source", "source_inputs", "duration_ms", "complete_transcript_inputs", "transcript_content_sha256", "generation_contexts", "chunk_scans", "explicit_sponsors"} {
			if value, ok := previous[key]; ok {
				inputs[key] = value
			}
		}
	}
	saveInputs := func() error {
		data, err := json.Marshal(inputs)
		if err != nil {
			return err
		}
		owned, err := p.db.SetPodcastProcessingAnalysisInputs(ctx, job.ID, job.Token, analysis.ID, data)
		if err != nil || !owned {
			return processingError("lease_lost", err)
		}
		return nil
	}
	if err := saveInputs(); err != nil {
		return err
	}
	transcripts := make(chan podcastTranscriptChunk, 24)
	type preparation struct {
		content database.PodcastTranscriptContent
		err     error
	}
	prepared := make(chan preparation, 1)
	prepStart := time.Now()
	workers.Add(1)
	go func() {
		defer workers.Done()
		content, _, err := p.prepareChunks(ctx, source, func(chunk podcastTranscriptChunk) error {
			select {
			case transcripts <- chunk:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		prepared <- preparation{content, err}
	}()
	notesReady := make(chan string, 1)
	notesStarted := false
	notesCacheable := false
	extractSponsors := p.extractSponsors
	if extractSponsors == nil {
		extractSponsors = extractPodcastSponsors
	}
	startNotes := func() {
		notesStarted = true
		notesCacheable = true
		result := notesReady
		workers.Add(1)
		go func() { defer workers.Done(); result <- extractSponsors(ctx, source.video.Description) }()
	}
	if previousNotes, ok := inputs["explicit_sponsors"]; ok {
		data, _ := json.Marshal(previousNotes)
		var sponsors []string
		if json.Unmarshal(data, &sponsors) == nil {
			notesReady <- string(data)
			notesStarted = true
			notesCacheable = true
		}
	}
	var notes string
	notesDone, prepDone := false, false
	var prepErr, scanErr error
	var completeContent *database.PodcastTranscriptContent
	recordCompleteInputs := func() {
		if completeContent != nil {
			inputs["transcript_content_sha256"] = completeContent.ContentHash
			inputs["complete_transcript_inputs"] = json.RawMessage(completeContent.InputJSON)
		}
	}
	var chunks []podcastTranscriptChunk
	var available []bool
	var cores []podcastCoreState
	var calls atomic.Int32
	limiter := make(chan struct{}, segmentWorkers)
	events := make(chan podcastScanEvent, 128)
	active := 0
	var scanStart time.Time
	defer func() {
		if !scanStart.IsZero() {
			outcome := "ready"
			if resultErr != nil {
				outcome = "failed"
			}
			metrics.ObservePodcastProcessingStage("scanning", outcome, time.Since(scanStart).Seconds())
		}
	}()
	proofs, _ := inputs["generation_contexts"].(map[string]any)
	if proofs == nil {
		proofs = map[string]any{}
	}
	firstPublished := false
	// Existing proven intervals survive an unrelated later failure or retry.
	accepted := append([]database.PodcastSegment{}, analysis.Segments...)
	lastPublished := ""
	publish := func() error {
		all := append([]database.PodcastSegment{}, accepted...)
		for i, core := range cores {
			for _, piece := range core.Pieces {
				settled := !piece.AwaitCore || core.Complete
				if piece.FirstChunk != i || piece.LastChunk != i {
					for j := piece.FirstChunk; j <= piece.LastChunk; j++ {
						if !cores[j].Complete {
							settled = false
							break
						}
					}
				}
				if settled {
					all = append(all, piece.Segments...)
				}
			}
		}
		all = mergePodcastSegments(all)
		data, _ := json.Marshal(all)
		hash := podcastSHA256(data)
		if hash == lastPublished {
			return nil
		}
		ok, err := p.db.PublishPodcastSegmentAnalysis(ctx, job.ID, job.Token, analysis.ID, all)
		if err != nil || !ok {
			return processingError("lease_lost", err)
		}
		accepted = all
		lastPublished = hash
		if !firstPublished && len(all) > 0 {
			firstPublished = true
			metrics.ObservePodcastProcessingStage("first_confirmed", "ready", time.Since(prepStart).Seconds())
		}
		return nil
	}
	for {
		// Silence needs no sponsor hints. Wait until preparation and its queued
		// chunk events finish before declaring the entire transcript silent.
		if prepDone && len(transcripts) == 0 && !notesStarted {
			notesStarted = true
			notesCacheable = prepErr == nil
			notesReady <- "[]"
		}
		if notesDone {
			for i := range cores {
				core := &cores[i]
				// Reserve model capacity for confirmation of earlier cores instead
				// of filling its queue with every later discovery batch.
				if active >= podcastCoreWorkers {
					break
				}
				if core.Running || core.Complete || core.Failed {
					continue
				}
				input, ready, e := makePodcastScanInput(chunks, available, i, core.First, core.Last)
				if e != nil {
					return processingError("transcript_timing_invalid", e)
				}
				if !ready {
					continue
				}
				input.Calls = &calls
				input.Limiter = limiter
				core.CacheKey, _, core.ContextHash = podcastScanHashes(input, source)
				proofs[core.CacheKey+":"+core.ContextHash] = map[string]any{"core_index": i, "scan_key": core.CacheKey, "context_sha256": core.ContextHash, "first_chunk": core.First, "last_chunk": core.Last}
				inputs["generation_contexts"] = proofs
				if err := saveInputs(); err != nil {
					return err
				}
				core.Running = true
				active++
				if scanStart.IsZero() {
					scanStart = time.Now()
				}
				ok, e = p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", aggregateHash, analysis.ID, input.DurationMS)
				if e != nil || !ok {
					return processingError("lease_lost", e)
				}
				workers.Add(1)
				go func() { defer workers.Done(); p.scanProgressiveCore(ctx, source, input, notes, events) }()
			}
		}
		// prepare sends chunk events before completion. Drain them before deciding
		// that no further core is eligible, even if select received completion first.
		if prepDone && notesDone && active == 0 && len(transcripts) == 0 {
			if prepErr != nil {
				return prepErr
			}
			if scanErr != nil {
				return processingError("model_output_invalid", scanErr)
			}
			complete := len(cores) > 0
			for _, core := range cores {
				complete = complete && core.Complete
			}
			if complete {
				if err := publish(); err != nil {
					return err
				}
				ok, err := p.db.CompletePodcastProcessingAnalysis(ctx, job.ID, job.Token, analysis.ID, accepted)
				if err != nil || !ok {
					return processingError("lease_lost", err)
				}
				categories := make([]string, 0, len(accepted))
				for _, segment := range accepted {
					categories = append(categories, segment.Category)
				}
				metrics.ObservePodcastSegmentAnalysis("ready", time.Since(scanStart).Seconds(), categories)
				return nil
			}
			return processingError("context_unresolved", nil)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case notes = <-notesReady:
			if notes == "" {
				notes = "[]"
			}
			notesDone = true
			notesReady = nil
			if notesCacheable {
				inputs["explicit_sponsors"] = json.RawMessage(notes)
				if err := saveInputs(); err != nil {
					return err
				}
			}
		case result := <-prepared:
			prepDone = true
			prepErr = result.err
			prepared = nil
			outcome := "ready"
			if result.err != nil {
				outcome = "failed"
			}
			metrics.ObservePodcastProcessingStage("preparing", outcome, time.Since(prepStart).Seconds())
			if result.err == nil {
				completeContent = &result.content
				if len(chunks) > 0 {
					recordCompleteInputs()
					if err := saveInputs(); err != nil {
						return err
					}
				}
			}
		case chunk := <-transcripts:
			if len(chunks) == 0 {
				var current struct {
					AudioHash string `json:"audio_sha256"`
				}
				_ = json.Unmarshal(chunk.InputJSON, &current)
				if previousSource.Source == "generated" && chunk.Source == "generated" && previousSource.Inputs.AudioHash != "" && current.AudioHash != "" && previousSource.Inputs.AudioHash != current.AudioHash {
					// Samples make playback validation cheap; the full download
					// can still reveal changes outside those samples on a retry.
					// Remove old intervals before recording any new generation.
					accepted = nil
					proofs = map[string]any{}
					for _, key := range []string{"generation_contexts", "chunk_scans", "complete_transcript_inputs", "transcript_content_sha256"} {
						delete(inputs, key)
					}
					if err := publish(); err != nil {
						return err
					}
				}
				chunks = make([]podcastTranscriptChunk, len(chunk.Chunks))
				available = make([]bool, len(chunks))
				cores = make([]podcastCoreState, len(chunks))
				for i := range cores {
					cores[i].First = max(0, i-1)
					cores[i].Last = min(len(cores)-1, i+1)
				}
				inputs["transcript_source"] = chunk.Source
				inputs["source_inputs"] = json.RawMessage(chunk.InputJSON)
				inputs["duration_ms"] = chunk.DurationMS
				inputs["total_chunks"] = len(chunks)
				recordCompleteInputs()
				if err := saveInputs(); err != nil {
					return err
				}
			}
			if chunk.Index < 0 || chunk.Index >= len(chunks) || len(chunk.Chunks) != len(chunks) || chunk.DurationMS <= 0 || chunk.DurationMS > maxPodcastDurationMS {
				return processingError("transcript_timing_invalid", nil)
			}
			chunks[chunk.Index] = chunk
			available[chunk.Index] = true
			if !notesStarted && (len(chunk.Cues) > 0 || len(chunk.Result.Segments) > 0) {
				startNotes()
			}
			phase := "preparing"
			if !scanStart.IsZero() {
				phase = "scanning"
			}
			ok, e := p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, phase, aggregateHash, analysis.ID, chunk.DurationMS)
			if e != nil || !ok {
				return processingError("lease_lost", e)
			}
		case event := <-events:
			core := &cores[event.Index]
			if event.Piece != nil {
				core.Pieces = append(core.Pieces, *event.Piece)
			} else {
				core.Running = false
				active--
				var deferred *podcastContextDeferred
				if errors.As(event.Err, &deferred) {
					first, last := core.First, core.Last
					if deferred.Left {
						first = max(0, first-1)
					}
					if deferred.Right {
						last = min(len(cores)-1, last+1)
					}
					if deferred.FirstChunk != nil {
						first = min(first, *deferred.FirstChunk)
					}
					if deferred.LastChunk != nil {
						last = max(last, *deferred.LastChunk)
					}
					if first == core.First && last == core.Last {
						core.Failed = true
						scanErr = event.Err
					} else {
						core.First = first
						core.Last = last
					}
				} else if event.Err != nil {
					core.Failed = true
					scanErr = event.Err
				} else {
					core.Complete = true
					core.Pieces = append(core.Pieces, event.Pieces...)
				}
			}
			var manifests []map[string]any
			for i, c := range cores {
				if c.CacheKey != "" {
					manifests = append(manifests, map[string]any{"core_index": i, "scan_key": c.CacheKey, "context_sha256": c.ContextHash, "first_chunk": c.First, "last_chunk": c.Last, "complete": c.Complete})
				}
			}
			inputs["chunk_scans"] = manifests
			if err := saveInputs(); err != nil {
				return err
			}
			if err := publish(); err != nil {
				return err
			}
		}
	}
}
