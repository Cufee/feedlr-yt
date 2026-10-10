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
	status := PodcastSegmentStatus{Status: "idle"}
	content, err := db.GetPodcastTranscriptContent(ctx, videoID, input.transcriptKey)
	if err == nil {
		status.Source = content.Source
		status.DurationMS = content.DurationMS
		status.TranscriptReady = content.Source == "generated" || content.Source == "publisher"
	} else if !errors.Is(err, sql.ErrNoRows) {
		return PodcastSegmentStatus{}, err
	}
	job, err := db.GetPodcastProcessingJob(ctx, videoID, input.jobKey, podcastModel(), podcastSegmentsPromptVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	status.Status, status.Phase, status.Error = job.Status, job.Phase, job.Error
	if content.Source == "" {
		status.DurationMS = job.DurationMS
	}
	switch job.Error {
	case "source_changed", "source_validation_failed", "configuration_changed":
		// These failures invalidate the provenance of retained intervals.
		return status, nil
	}
	if job.AnalysisID != "" {
		analysis, err := db.GetPodcastSegmentAnalysis(ctx, videoID, job.TranscriptHash, job.Model, job.PromptVersion)
		if err != nil {
			return PodcastSegmentStatus{}, err
		}
		if analysis.ID == job.AnalysisID {
			status.Segments = analysis.Segments
			if status.Source == "" {
				data, err := db.GetPodcastSegmentAnalysisInputs(ctx, analysis.ID)
				if err != nil {
					return PodcastSegmentStatus{}, err
				}
				var details struct {
					Source string `json:"transcript_source"`
				}
				_ = json.Unmarshal(data, &details)
				status.Source = details.Source
			}
		}
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
	p := &podcastProcessor{db: db, generateChunks: generatePodcastTranscriptChunks, scanChunk: inferPodcastChunkSegments}
	for i := 0; i < 2; i++ {
		go p.work()
	}
}

type podcastProcessor struct {
	db              database.Client
	generate        func(context.Context, database.Client, podcastInput) (database.PodcastTranscriptContent, error)
	generateChunks  func(context.Context, database.Client, podcastInput, func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error)
	scanChunk       podcastChunkScanner
	extractSponsors func(context.Context, string) string
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
	if err := p.processProgressive(ctx, job, input); err != nil {
		fail(err)
	} else {
		finish(database.PodcastSegmentReady, "")
	}
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
	return p.prepareChunks(ctx, input, nil)
}

func (p *podcastProcessor) prepareChunks(ctx context.Context, input podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, []transcriptCue, error) {
	cached, err := p.db.GetPodcastTranscriptContent(ctx, input.video.ID, input.transcriptKey)
	if err == nil {
		var cues []transcriptCue
		if json.Unmarshal(cached.CuesJSON, &cues) != nil || validateTranscriptCues(cues, cached.DurationMS, cached.Source == "generated") != nil {
			return cached, nil, processingError("cached_transcript_invalid", nil)
		}
		if ready != nil {
			if err := emitPodcastTranscriptChunks(cached, cues, ready); err != nil {
				return cached, nil, err
			}
		}
		return cached, cues, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return cached, nil, err
	}
	var content database.PodcastTranscriptContent
	var cues []transcriptCue
	streamed := false
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
		if p.generateChunks != nil {
			content, err = p.generateChunks(ctx, p.db, input, ready)
			streamed = ready != nil
		} else {
			content, err = p.generate(ctx, p.db, input)
		}
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
	if ready != nil && !streamed {
		if err := emitPodcastTranscriptChunks(content, cues, ready); err != nil {
			return content, nil, err
		}
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
