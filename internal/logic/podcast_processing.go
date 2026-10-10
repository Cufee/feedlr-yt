package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/lucsky/cuid"
)

const podcastJobLease = 2 * time.Minute
const maxPodcastDurationMS = 4 * 60 * 60 * 1000

func podcastTranscriptionEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PODCAST_TRANSCRIPTION_ENABLED")), "true")
}

type podcastInput struct {
	video                 *models.Video
	publisher             database.PodcastTranscript
	metadataKey           string
	validation            database.PodcastSourceValidation
	transcriptKey, jobKey string
}

// Keys use source identity, not the feed refresh time. Notes affect scanning,
// while changes to enclosure or publisher metadata invalidate transcript input.
func loadPodcastInput(ctx context.Context, db database.Client, videoID string) (podcastInput, error) {
	v, err := db.GetVideoByID(ctx, videoID)
	if err != nil {
		return podcastInput{}, err
	}
	if v.Type != string(youtube.VideoTypePodcastEpisode) {
		return podcastInput{}, errors.New("video is not a podcast episode")
	}
	source, err := db.GetPodcastTranscript(ctx, videoID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return podcastInput{}, err
	}
	metadata, _ := json.Marshal([]any{stablePodcastSourceURL(v.MediaURL.String), v.Duration, stablePodcastSourceURL(source.URL), source.MIMEType, source.Language, source.Rel})
	input := podcastInput{video: v, publisher: source, metadataKey: podcastSHA256(metadata)}
	input.validation, err = loadPodcastValidation(ctx, db, videoID, input.metadataKey)
	if err != nil {
		return podcastInput{}, err
	}
	input.setKeys()
	return input, nil
}

func (input *podcastInput) setKeys() {
	input.transcriptKey = hashTranscript(nil, input.metadataKey, input.validation.Fingerprint, "podcast-transcript-v3", openrouter.TranscriptionModel)
	input.jobKey = hashTranscript(nil, input.transcriptKey, "", input.video.Description, input.video.Title)
}

// GetPodcastSegmentStatus only reads local cached state. It never downloads or
// contacts a provider, even for a missing or expired job.
func GetPodcastSegmentStatus(ctx context.Context, db database.Client, videoID string) (PodcastSegmentStatus, error) {
	input, err := loadPodcastInput(ctx, db, videoID)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	job, err := db.GetPodcastProcessingJob(ctx, videoID, input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return PodcastSegmentStatus{Status: "idle"}, nil
	}
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	status := PodcastSegmentStatus{Status: job.Status, Phase: job.Phase, Error: job.Error, DurationMS: job.DurationMS}
	content, err := db.GetPodcastTranscriptContent(ctx, videoID, input.transcriptKey)
	if err == nil {
		status.Source = content.Source
		status.DurationMS = content.DurationMS
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PodcastSegmentStatus{}, err
	}
	if job.Status == database.PodcastSegmentReady {
		analysis, err := db.GetPodcastSegmentAnalysis(ctx, videoID, job.TranscriptHash, job.Model, job.PromptVersion)
		if err != nil {
			return PodcastSegmentStatus{}, err
		}
		status.Segments = analysis.Segments
	}
	return status, nil
}

// EnsurePodcastSegmentAnalysis is called exclusively by the authenticated,
// settings-gated POST handler. Enqueue is durable; listeners may leave immediately.
func EnsurePodcastSegmentAnalysis(ctx context.Context, db database.Client, videoID string) (PodcastSegmentStatus, error) {
	input, err := loadPodcastInput(ctx, db, videoID)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	// An uncached disabled provider requires no origin traffic. Cached results
	// still require validation before they can be used for automatic skipping.
	cached, err := GetPodcastSegmentStatus(ctx, db, videoID)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	if cached.Status != database.PodcastSegmentReady && openrouter.DefaultClient == nil {
		return PodcastSegmentStatus{Status: database.PodcastSegmentUnavailable, Error: "provider_disabled"}, nil
	}
	input, err = revalidatePodcastInput(ctx, db, input, cached.Error == "source_changed")
	if err != nil {
		return PodcastSegmentStatus{Status: database.PodcastSegmentFailed, Error: "source_validation_failed"}, nil
	}
	cached, err = GetPodcastSegmentStatus(ctx, db, videoID)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	if cached.Status == database.PodcastSegmentReady {
		return cached, nil
	}
	if openrouter.DefaultClient == nil {
		return PodcastSegmentStatus{Status: database.PodcastSegmentUnavailable, Error: "provider_disabled"}, nil
	}
	if _, err = db.EnqueuePodcastProcessingJob(ctx, videoID, input.jobKey, podcastModel(), podcastSegmentsPromptVersion); err != nil {
		return PodcastSegmentStatus{}, err
	}
	StartPodcastProcessingWorkers(db)
	return GetPodcastSegmentStatus(ctx, db, videoID)
}

var podcastProcessors sync.Map

// Started at application startup as well as kickoff, so accepted jobs recover
// without needing another listener or a polling request after a crash.
func StartPodcastProcessingWorkers(db database.Client) {
	if openrouter.DefaultClient == nil {
		return
	}
	if _, exists := podcastProcessors.LoadOrStore(db, struct{}{}); exists {
		return
	}
	p := &podcastProcessor{db: db, generate: generatePodcastTranscript, scan: inferPodcastSegments}
	for i := 0; i < 2; i++ {
		go p.work()
	}
}

type podcastProcessor struct {
	db       database.Client
	generate func(context.Context, database.Client, podcastInput) (database.PodcastTranscriptContent, error)
	scan     func(context.Context, []transcriptCue, string, string) ([]database.PodcastSegment, error)
}

func (p *podcastProcessor) work() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		job, claimed, err := p.db.ClaimPodcastProcessingJob(ctx, cuid.New(), podcastJobLease)
		cancel()
		if err != nil {
			log.Printf("podcast job claim: %v", err)
		}
		if claimed && err == nil {
			p.run(job)
			continue
		}
		time.Sleep(time.Second)
	}
}

func (p *podcastProcessor) run(job database.PodcastProcessingJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	done := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				ok, err := p.db.RenewPodcastProcessingJob(ctx, job.ID, job.Token, podcastJobLease)
				if err != nil || !ok {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { close(done); <-heartbeatDone }()
	started := time.Now()
	finish := func(status, failure string) {
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		if _, err := p.db.FinishPodcastProcessingJob(cctx, job.ID, job.Token, status, failure); err != nil {
			log.Printf("podcast job completion: %v", err)
		}
		metrics.ObservePodcastProcessingStage("total", status, time.Since(started).Seconds())
	}
	fail := func(err error) {
		code := "processing_failed"
		var failure *podcastProcessingError
		if errors.As(err, &failure) {
			code = failure.code
		}
		log.Printf("podcast job %s: %v", job.ID, err)
		finish(database.PodcastSegmentFailed, code)
	}
	input, err := loadPodcastInput(ctx, p.db, job.VideoID)
	if err != nil {
		fail(err)
		return
	}
	if input.jobKey != job.SourceKey {
		fail(processingError("source_changed", nil))
		return
	}
	if job.Model != podcastModel() || job.PromptVersion != podcastSegmentsPromptVersion {
		fail(processingError("configuration_changed", nil))
		return
	}
	if input.video.Duration*1000 > maxPodcastDurationMS {
		fail(processingError("episode_too_long", nil))
		return
	}
	preparationStart := time.Now()
	owned, err := p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "preparing", job.TranscriptHash, job.AnalysisID, job.DurationMS)
	if err != nil || !owned {
		fail(processingError("lease_lost", err))
		return
	}
	content, cues, err := p.prepare(ctx, input)
	if err != nil {
		fail(err)
		return
	}
	metrics.ObservePodcastProcessingStage("preparing", "ready", time.Since(preparationStart).Seconds())
	hash := hashTranscript(content.CuesJSON, input.transcriptKey, "", input.video.Description, input.video.Title)
	ok, err := p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", hash, "", content.DurationMS)
	if err != nil || !ok {
		fail(processingError("lease_lost", err))
		return
	}
	analysis, err := p.db.GetPodcastSegmentAnalysis(ctx, job.VideoID, hash, job.Model, job.PromptVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		fail(err)
		return
	}
	if err == nil && analysis.Status == database.PodcastSegmentReady {
		finish(database.PodcastSegmentReady, "")
		return
	}
	// This durable job's lease is the owner for this source and scan input.
	// A failed/interrupted analysis can be reused and overwritten by its successor.
	analysis, _, err = p.db.AcquirePodcastSegmentAnalysis(ctx, job.VideoID, hash, content.SourceURL, job.Model, job.PromptVersion)
	if err != nil {
		fail(err)
		return
	}
	analysisInputs, _ := json.Marshal(map[string]any{
		"version": podcastInputVersion, "transcript_source_key": content.SourceKey, "transcript_content_sha256": content.ContentHash,
		"scan_input_hash": hash, "source_fingerprint": input.validation.Fingerprint,
		"title_sha256": podcastSHA256([]byte(input.video.Title)), "notes_sha256": podcastSHA256([]byte(input.video.Description)),
		"model": job.Model, "prompt_version": job.PromptVersion,
		"prompts_sha256": podcastSHA256([]byte(segmentDiscoveryPrompt + "\x00" + segmentConfirmationPrompt + "\x00" + segmentBoundaryReviewPrompt + "\x00" + segmentResponsePrompt + "\x00" + sponsorExtractionPrompt)),
	})
	if err = p.db.SetPodcastSegmentAnalysisInputs(ctx, analysis.ID, analysisInputs); err != nil {
		fail(err)
		return
	}
	ok, err = p.db.UpdatePodcastProcessingJob(ctx, job.ID, job.Token, "scanning", hash, analysis.ID, content.DurationMS)
	if err != nil || !ok {
		fail(processingError("lease_lost", err))
		return
	}
	scanStart := time.Now()
	var segments []database.PodcastSegment
	if len(cues) > 0 {
		notes := extractPodcastSponsors(ctx, input.video.Description)
		segments, err = p.scan(ctx, cues, notes, input.video.Title)
	}
	outcome := "ready"
	if err != nil {
		outcome = "failed"
	}
	metrics.ObservePodcastProcessingStage("scanning", outcome, time.Since(scanStart).Seconds())
	// Renew before persisting results so a superseded worker cannot complete a job.
	owned, leaseErr := p.db.RenewPodcastProcessingJob(ctx, job.ID, job.Token, podcastJobLease)
	if leaseErr != nil || !owned {
		fail(processingError("lease_lost", leaseErr))
		return
	}
	if err != nil {
		_ = p.db.CompletePodcastSegmentAnalysis(ctx, analysis.ID, database.PodcastSegmentFailed, "model_output_invalid", nil)
		fail(processingError("model_output_invalid", err))
		return
	}
	if err = p.db.CompletePodcastSegmentAnalysis(ctx, analysis.ID, database.PodcastSegmentReady, "", segments); err != nil {
		fail(err)
		return
	}
	categories := make([]string, 0, len(segments))
	for _, segment := range segments {
		categories = append(categories, segment.Category)
	}
	metrics.ObservePodcastSegmentAnalysis("ready", time.Since(scanStart).Seconds(), categories)
	finish(database.PodcastSegmentReady, "")
}

type podcastProcessingError struct {
	code  string
	cause error
}

func (e *podcastProcessingError) Error() string { return fmt.Sprintf("%s: %v", e.code, e.cause) }
func (e *podcastProcessingError) Unwrap() error { return e.cause }
func processingError(code string, err error) error {
	return &podcastProcessingError{code: code, cause: err}
}

func (p *podcastProcessor) prepare(ctx context.Context, input podcastInput) (database.PodcastTranscriptContent, []transcriptCue, error) {
	cached, err := p.db.GetPodcastTranscriptContent(ctx, input.video.ID, input.transcriptKey)
	if err == nil {
		var cues []transcriptCue
		if json.Unmarshal(cached.CuesJSON, &cues) != nil || validateTranscriptCues(cues, cached.DurationMS, cached.Source == "generated") != nil {
			return cached, nil, processingError("cached_transcript_invalid", nil)
		}
		return cached, cues, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return cached, nil, err
	}
	var content database.PodcastTranscriptContent
	var cues []transcriptCue
	if input.publisher.URL != "" && isTimedTranscript(input.publisher.MIMEType) {
		data, failure := fetchTranscript(ctx, input.publisher.URL, input.publisher.MIMEType)
		if failure == "" {
			expected := podcastValidationInputs(input).Publisher.ContentHash
			if expected != "" && expected != podcastSHA256(data) {
				return content, nil, processingError("source_changed", nil)
			}
			parsed, parseErr := parseTimedTranscript(data)
			if parseErr == nil && validateTranscriptCues(parsed, 0, false) == nil {
				cues = parsed
				content = database.PodcastTranscriptContent{Source: "publisher", SourceURL: input.publisher.URL, DurationMS: int(input.video.Duration) * 1000, UsageJSON: []byte("{}")}
				content.InputJSON, _ = json.Marshal(map[string]any{"publisher_content_sha256": podcastSHA256(data)})
			}
		}
	}
	if content.Source == "" {
		if !podcastTranscriptionEnabled() {
			return content, nil, processingError("transcription_disabled", nil)
		}
		content, err = p.generate(ctx, p.db, input)
		if err != nil {
			return content, nil, err
		}
		if err = json.Unmarshal(content.CuesJSON, &cues); err != nil {
			return content, nil, processingError("transcription_output_invalid", err)
		}
	}
	if err = validateTranscriptCues(cues, content.DurationMS, content.Source == "generated"); err != nil {
		return content, nil, processingError("transcript_timing_invalid", err)
	}
	content.VideoID = input.video.ID
	content.SourceKey = input.transcriptKey
	content.CuesJSON, err = json.Marshal(cues)
	if err != nil {
		return content, nil, err
	}
	content.ContentHash = podcastSHA256(content.CuesJSON)
	var details map[string]any
	_ = json.Unmarshal(content.InputJSON, &details)
	if details == nil {
		details = make(map[string]any)
	}
	details["version"], details["metadata_key"], details["source_key"] = podcastInputVersion, input.metadataKey, input.transcriptKey
	details["source_fingerprint"], details["source_inputs"] = input.validation.Fingerprint, podcastValidationInputs(input)
	details["validated_at"], details["model"], details["duration_ms"] = input.validation.ValidatedAt, content.Model, content.DurationMS
	details["transcript_content_sha256"] = content.ContentHash
	content.InputJSON, err = json.Marshal(details)
	if err != nil {
		return content, nil, err
	}
	if err = p.db.SavePodcastTranscriptContent(ctx, content); err != nil {
		return content, nil, err
	}
	return content, cues, nil
}

func validateTranscriptCues(cues []transcriptCue, durationMS int, generated bool) error {
	if generated && (durationMS <= 0 || durationMS > maxPodcastDurationMS) {
		return errors.New("invalid audio duration")
	}
	if !generated && len(cues) == 0 {
		return errors.New("empty publisher transcript")
	}
	lastStart := -1
	for i, cue := range cues {
		if cue.Index != i || cue.StartMS < 0 || cue.StartMS < lastStart || cue.EndMS <= cue.StartMS || cue.EndMS > maxPodcastDurationMS || strings.TrimSpace(cue.Text) == "" {
			return errors.New("invalid cue timing or text")
		}
		if generated && cue.EndMS > durationMS {
			return errors.New("cue exceeds audio duration")
		}
		lastStart = cue.StartMS
	}
	return nil
}
