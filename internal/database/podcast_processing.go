package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lucsky/cuid"
)

// PodcastTranscriptContent is the complete, reusable transcript for a particular
// source metadata version. Publisher transcript metadata lives separately.
type PodcastTranscriptContent struct {
	VideoID, SourceKey, Source, SourceURL, ContentHash, Model string
	DurationMS                                                int
	CuesJSON, UsageJSON, InputJSON                            []byte
}

// PodcastSourceValidation records lightweight origin validators separately
// from the durable transcript, so a playback kickoff can revalidate cheaply.
type PodcastSourceValidation struct {
	VideoID, MetadataKey, Fingerprint string
	InputJSON                         []byte
	ValidatedAt                       time.Time
}

type PodcastProcessingJob struct {
	ID, VideoID, SourceKey, Model, PromptVersion, Status, Phase, Error, TranscriptHash, AnalysisID string
	DurationMS                                                                                     int
	Token                                                                                          string
}

type PodcastProcessingClient interface {
	GetPodcastTranscriptContent(context.Context, string, string) (PodcastTranscriptContent, error)
	SavePodcastTranscriptContent(context.Context, PodcastTranscriptContent) error
	GetPodcastSourceValidation(context.Context, string, string) (PodcastSourceValidation, error)
	SavePodcastSourceValidation(context.Context, PodcastSourceValidation) error
	SetPodcastSegmentAnalysisInputs(context.Context, string, []byte) error
	GetPodcastSegmentAnalysisInputs(context.Context, string) ([]byte, error)
	EnqueuePodcastProcessingJob(context.Context, string, string, string, string) (PodcastProcessingJob, error)
	GetPodcastProcessingJob(context.Context, string, string, string, string) (PodcastProcessingJob, error)
	ClaimPodcastProcessingJob(context.Context, string, time.Duration) (PodcastProcessingJob, bool, error)
	RenewPodcastProcessingJob(context.Context, string, string, time.Duration) (bool, error)
	UpdatePodcastProcessingJob(context.Context, string, string, string, string, string, int) (bool, error)
	FinishPodcastProcessingJob(context.Context, string, string, string, string) (bool, error)
	AcquirePodcastTranscriptionSlot(context.Context, string, time.Duration) (bool, error)
	ReleasePodcastTranscriptionSlot(context.Context, string) error
}

func (c *sqliteClient) GetPodcastTranscriptContent(ctx context.Context, videoID, sourceKey string) (PodcastTranscriptContent, error) {
	var v PodcastTranscriptContent
	err := c.db.QueryRowContext(ctx, `SELECT video_id, source_key, source, source_url, content_hash, model, duration_ms, cues_json, usage_json, input_json
		FROM podcast_transcript_contents WHERE video_id = ? AND source_key = ?`, videoID, sourceKey).
		Scan(&v.VideoID, &v.SourceKey, &v.Source, &v.SourceURL, &v.ContentHash, &v.Model, &v.DurationMS, &v.CuesJSON, &v.UsageJSON, &v.InputJSON)
	return v, err
}

func (c *sqliteClient) SavePodcastTranscriptContent(ctx context.Context, v PodcastTranscriptContent) error {
	usage := v.UsageJSON
	if len(usage) == 0 {
		usage = []byte("{}")
	}
	inputs := v.InputJSON
	if len(inputs) == 0 {
		inputs = []byte("{}")
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO podcast_transcript_contents
		(video_id, source_key, source, source_url, content_hash, model, duration_ms, cues_json, usage_json, input_json, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(video_id, source_key) DO UPDATE SET
		 source = excluded.source, source_url = excluded.source_url, content_hash = excluded.content_hash,
		 model = excluded.model, duration_ms = excluded.duration_ms, cues_json = excluded.cues_json,
		 usage_json = excluded.usage_json, input_json = excluded.input_json, updated_at_ms = excluded.updated_at_ms`,
		v.VideoID, v.SourceKey, v.Source, v.SourceURL, v.ContentHash, v.Model, v.DurationMS, v.CuesJSON, usage, inputs, time.Now().UnixMilli())
	return err
}

func (c *sqliteClient) GetPodcastSourceValidation(ctx context.Context, videoID, metadataKey string) (PodcastSourceValidation, error) {
	var v PodcastSourceValidation
	err := c.db.QueryRowContext(ctx, `SELECT video_id, metadata_key, fingerprint, input_json, validated_at
		FROM podcast_source_validations WHERE video_id = ? AND metadata_key = ?`, videoID, metadataKey).
		Scan(&v.VideoID, &v.MetadataKey, &v.Fingerprint, &v.InputJSON, &v.ValidatedAt)
	return v, err
}

func (c *sqliteClient) SavePodcastSourceValidation(ctx context.Context, v PodcastSourceValidation) error {
	inputs := v.InputJSON
	if len(inputs) == 0 {
		inputs = []byte("{}")
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO podcast_source_validations(video_id, metadata_key, fingerprint, input_json, validated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(video_id, metadata_key) DO UPDATE SET
		 fingerprint = excluded.fingerprint, input_json = excluded.input_json, validated_at = excluded.validated_at
		WHERE excluded.validated_at >= podcast_source_validations.validated_at`,
		v.VideoID, v.MetadataKey, v.Fingerprint, inputs, v.ValidatedAt.UTC())
	return err
}

func (c *sqliteClient) SetPodcastSegmentAnalysisInputs(ctx context.Context, id string, inputs []byte) error {
	if len(inputs) == 0 {
		inputs = []byte("{}")
	}
	changed, err := podcastProcessingChanged(c.db.ExecContext(ctx, `UPDATE podcast_segment_analyses SET input_json = ? WHERE id = ?`, inputs, id))
	if err == nil && !changed {
		return sql.ErrNoRows
	}
	return err
}

func (c *sqliteClient) GetPodcastSegmentAnalysisInputs(ctx context.Context, id string) ([]byte, error) {
	var inputs []byte
	err := c.db.QueryRowContext(ctx, `SELECT input_json FROM podcast_segment_analyses WHERE id = ?`, id).Scan(&inputs)
	return inputs, err
}

const podcastProcessingJobColumns = `id, video_id, source_key, model, prompt_version, status, phase, error, transcript_hash, analysis_id, duration_ms, token`

func scanPodcastProcessingJob(row *sql.Row) (PodcastProcessingJob, error) {
	var v PodcastProcessingJob
	err := row.Scan(&v.ID, &v.VideoID, &v.SourceKey, &v.Model, &v.PromptVersion, &v.Status, &v.Phase,
		&v.Error, &v.TranscriptHash, &v.AnalysisID, &v.DurationMS, &v.Token)
	return v, err
}

func (c *sqliteClient) EnqueuePodcastProcessingJob(ctx context.Context, videoID, sourceKey, model, prompt string) (PodcastProcessingJob, error) {
	now := time.Now().UnixMilli()
	// Only this explicit enqueue operation retries terminal failures. Concurrent
	// callers retain the same job, including an active worker's token and lease.
	return scanPodcastProcessingJob(c.db.QueryRowContext(ctx, `INSERT INTO podcast_processing_jobs
		(id, video_id, source_key, model, prompt_version, status, phase, error, transcript_hash, analysis_id, duration_ms, token, lease_until_ms, created_at_ms, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, 'pending', 'queued', '', '', '', 0, '', 0, ?, ?)
		ON CONFLICT(video_id, source_key, model, prompt_version) DO UPDATE SET
		 status = CASE WHEN status IN ('failed', 'unavailable') THEN 'pending' ELSE status END,
		 phase = CASE WHEN status IN ('failed', 'unavailable') THEN 'queued' ELSE phase END,
		 error = CASE WHEN status IN ('failed', 'unavailable') THEN '' ELSE error END,
		 analysis_id = CASE WHEN status IN ('failed', 'unavailable') THEN '' ELSE analysis_id END,
		 token = CASE WHEN status IN ('failed', 'unavailable') THEN '' ELSE token END,
		 lease_until_ms = CASE WHEN status IN ('failed', 'unavailable') THEN 0 ELSE lease_until_ms END,
		 updated_at_ms = CASE WHEN status IN ('failed', 'unavailable') THEN excluded.updated_at_ms ELSE updated_at_ms END
		RETURNING `+podcastProcessingJobColumns, cuid.New(), videoID, sourceKey, model, prompt, now, now))
}

func (c *sqliteClient) GetPodcastProcessingJob(ctx context.Context, videoID, sourceKey, model, prompt string) (PodcastProcessingJob, error) {
	return scanPodcastProcessingJob(c.db.QueryRowContext(ctx, `SELECT `+podcastProcessingJobColumns+`
		FROM podcast_processing_jobs WHERE video_id = ? AND source_key = ? AND model = ? AND prompt_version = ?`,
		videoID, sourceKey, model, prompt))
}

func podcastLeaseDeadline(token string, lease time.Duration) (int64, int64, error) {
	if token == "" || lease < time.Millisecond {
		return 0, 0, fmt.Errorf("podcast lease requires a nonempty token and duration of at least one millisecond")
	}
	now := time.Now().UnixMilli()
	return now, now + lease.Milliseconds(), nil
}

func (c *sqliteClient) ClaimPodcastProcessingJob(ctx context.Context, token string, lease time.Duration) (PodcastProcessingJob, bool, error) {
	now, until, err := podcastLeaseDeadline(token, lease)
	if err != nil {
		return PodcastProcessingJob{}, false, err
	}
	// Selection, capacity checking, and ownership assignment happen in one SQLite
	// write statement, so distinct processes cannot exceed the two-job limit.
	v, err := scanPodcastProcessingJob(c.db.QueryRowContext(ctx, `UPDATE podcast_processing_jobs
		SET status = 'running', token = ?, lease_until_ms = ?, updated_at_ms = ?
		WHERE id = (SELECT id FROM podcast_processing_jobs
		 WHERE status = 'pending' OR (status = 'running' AND lease_until_ms <= ?)
		 ORDER BY created_at_ms, id LIMIT 1)
		AND (SELECT COUNT(*) FROM podcast_processing_jobs WHERE status = 'running' AND lease_until_ms > ?) < 2
		RETURNING `+podcastProcessingJobColumns, token, until, now, now, now))
	if errors.Is(err, sql.ErrNoRows) {
		return PodcastProcessingJob{}, false, nil
	}
	return v, err == nil, err
}

func (c *sqliteClient) RenewPodcastProcessingJob(ctx context.Context, id, token string, lease time.Duration) (bool, error) {
	now, until, err := podcastLeaseDeadline(token, lease)
	if err != nil {
		return false, err
	}
	return podcastProcessingChanged(c.db.ExecContext(ctx, `UPDATE podcast_processing_jobs SET lease_until_ms = ?, updated_at_ms = ?
		WHERE id = ? AND status = 'running' AND token = ? AND lease_until_ms > ?`, until, now, id, token, now))
}

func (c *sqliteClient) UpdatePodcastProcessingJob(ctx context.Context, id, token, phase, hash, analysisID string, durationMS int) (bool, error) {
	now := time.Now().UnixMilli()
	return podcastProcessingChanged(c.db.ExecContext(ctx, `UPDATE podcast_processing_jobs
		SET phase = ?, transcript_hash = ?, analysis_id = ?, duration_ms = ?, updated_at_ms = ?
		WHERE id = ? AND status = 'running' AND token = ? AND lease_until_ms > ?`,
		phase, hash, analysisID, durationMS, now, id, token, now))
}

func (c *sqliteClient) FinishPodcastProcessingJob(ctx context.Context, id, token, status, failure string) (bool, error) {
	if status != PodcastSegmentReady && status != PodcastSegmentFailed && status != PodcastSegmentUnavailable {
		return false, fmt.Errorf("invalid terminal podcast processing status %q", status)
	}
	now := time.Now().UnixMilli()
	return podcastProcessingChanged(c.db.ExecContext(ctx, `UPDATE podcast_processing_jobs
		SET status = ?, phase = ?, error = ?, token = '', lease_until_ms = 0, updated_at_ms = ?
		WHERE id = ? AND status = 'running' AND token = ? AND lease_until_ms > ?`, status, status, failure, now, id, token, now))
}

func podcastProcessingChanged(result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// AcquirePodcastTranscriptionSlot also renews a slot held by this token. Tokens
// must be unique per request; callers can renew during a long transcription.
func (c *sqliteClient) AcquirePodcastTranscriptionSlot(ctx context.Context, token string, lease time.Duration) (bool, error) {
	now, until, err := podcastLeaseDeadline(token, lease)
	if err != nil {
		return false, err
	}
	if _, err := c.db.ExecContext(ctx, `DELETE FROM podcast_transcription_slots WHERE lease_until_ms <= ?`, now); err != nil {
		return false, err
	}
	return podcastProcessingChanged(c.db.ExecContext(ctx, `INSERT INTO podcast_transcription_slots(token, lease_until_ms)
		SELECT ?, ? WHERE (SELECT COUNT(*) FROM podcast_transcription_slots WHERE lease_until_ms > ?) < 4
		 OR EXISTS (SELECT 1 FROM podcast_transcription_slots WHERE token = ? AND lease_until_ms > ?)
		ON CONFLICT(token) DO UPDATE SET lease_until_ms = excluded.lease_until_ms`, token, until, now, token, now))
}

func (c *sqliteClient) ReleasePodcastTranscriptionSlot(ctx context.Context, token string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM podcast_transcription_slots WHERE token = ?`, token)
	return err
}
