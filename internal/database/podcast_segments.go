package database

import (
	"context"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/lucsky/cuid"
)

const (
	PodcastSegmentPending     = "pending"
	PodcastSegmentRunning     = "running"
	PodcastSegmentReady       = "ready"
	PodcastSegmentFailed      = "failed"
	PodcastSegmentUnavailable = "unavailable"
	PodcastSegmentSourceLLM   = "transcript_llm"
)

type PodcastTranscript struct{ VideoID, URL, MIMEType, Language, Rel string }
type PodcastSegment struct {
	Category, StartText, EndText, Reason, Brand string
	StartMS, EndMS, StartCue, EndCue            int
}
type PodcastSegmentAnalysis struct {
	ID, VideoID, TranscriptHash, TranscriptURL, Model, PromptVersion, Status, Error string
	StartedAt, CompletedAt                                                          null.Time
	Segments                                                                        []PodcastSegment
}

type PodcastSegmentsClient interface {
	UpsertPodcastTranscript(context.Context, PodcastTranscript) error
	GetPodcastTranscript(context.Context, string) (PodcastTranscript, error)
	GetPodcastSegmentAnalysis(context.Context, string, string, string, string) (PodcastSegmentAnalysis, error)
	AcquirePodcastSegmentAnalysis(context.Context, string, string, string, string, string) (PodcastSegmentAnalysis, bool, error)
	CompletePodcastSegmentAnalysis(context.Context, string, string, string, []PodcastSegment) error
	GetLatestPodcastSegmentAnalysis(context.Context, string) (PodcastSegmentAnalysis, error)
}

func (c *sqliteClient) UpsertPodcastTranscript(ctx context.Context, s PodcastTranscript) error {
	v := &models.PodcastEpisodeTranscript{VideoID: s.VideoID, URL: s.URL, MimeType: s.MIMEType, Language: null.StringFrom(s.Language), Rel: null.StringFrom(s.Rel), UpdatedAt: time.Now()}
	return v.Upsert(ctx, c.db, true, []string{models.PodcastEpisodeTranscriptColumns.VideoID}, boil.Blacklist(models.PodcastEpisodeTranscriptColumns.VideoID), boil.Infer())
}
func (c *sqliteClient) GetPodcastTranscript(ctx context.Context, videoID string) (PodcastTranscript, error) {
	v, err := models.FindPodcastEpisodeTranscript(ctx, c.db, videoID)
	if err != nil {
		return PodcastTranscript{}, err
	}
	return PodcastTranscript{VideoID: v.VideoID, URL: v.URL, MIMEType: v.MimeType, Language: v.Language.String, Rel: v.Rel.String}, nil
}
func mapAnalysis(v *models.PodcastSegmentAnalysis, rows models.PodcastEpisodeSegmentSlice) PodcastSegmentAnalysis {
	a := PodcastSegmentAnalysis{ID: v.ID, VideoID: v.VideoID, TranscriptHash: v.TranscriptHash, TranscriptURL: v.TranscriptURL, Model: v.Model, PromptVersion: v.PromptVersion, Status: v.Status, Error: v.Error.String, StartedAt: v.StartedAt, CompletedAt: v.CompletedAt}
	for _, r := range rows {
		a.Segments = append(a.Segments, PodcastSegment{Category: r.Category, StartMS: int(r.StartMS), EndMS: int(r.EndMS), StartCue: int(r.StartCue), EndCue: int(r.EndCue), StartText: r.StartText, EndText: r.EndText, Reason: r.Reason, Brand: r.Brand.String})
	}
	return a
}
func (c *sqliteClient) analysis(ctx context.Context, mods ...qm.QueryMod) (PodcastSegmentAnalysis, error) {
	v, err := models.PodcastSegmentAnalyses(mods...).One(ctx, c.db)
	if err != nil {
		return PodcastSegmentAnalysis{}, err
	}
	rows, err := models.PodcastEpisodeSegments(qm.Where(models.PodcastEpisodeSegmentColumns.AnalysisID+"=?", v.ID), qm.OrderBy(models.PodcastEpisodeSegmentColumns.Position)).All(ctx, c.db)
	if err != nil {
		return PodcastSegmentAnalysis{}, err
	}
	return mapAnalysis(v, rows), nil
}
func (c *sqliteClient) GetPodcastSegmentAnalysis(ctx context.Context, videoID, hash, model, prompt string) (PodcastSegmentAnalysis, error) {
	return c.analysis(ctx, models.PodcastSegmentAnalysisWhere.VideoID.EQ(videoID), models.PodcastSegmentAnalysisWhere.TranscriptHash.EQ(hash), models.PodcastSegmentAnalysisWhere.Model.EQ(model), models.PodcastSegmentAnalysisWhere.PromptVersion.EQ(prompt))
}
func (c *sqliteClient) GetLatestPodcastSegmentAnalysis(ctx context.Context, videoID string) (PodcastSegmentAnalysis, error) {
	return c.analysis(ctx, models.PodcastSegmentAnalysisWhere.VideoID.EQ(videoID), qm.OrderBy(models.PodcastSegmentAnalysisColumns.CreatedAt+" DESC"))
}

func (c *sqliteClient) AcquirePodcastSegmentAnalysis(ctx context.Context, videoID, hash, url, model, prompt string) (PodcastSegmentAnalysis, bool, error) {
	now := time.Now()
	v := &models.PodcastSegmentAnalysis{ID: cuid.New(), VideoID: videoID, TranscriptHash: hash, TranscriptURL: url, Model: model, PromptVersion: prompt, Status: PodcastSegmentPending, CreatedAt: now, UpdatedAt: now}
	if err := v.Insert(ctx, c.db, boil.Infer()); err != nil {
		a, e := c.GetPodcastSegmentAnalysis(ctx, videoID, hash, model, prompt)
		return a, false, e
	}
	v.Status = PodcastSegmentRunning
	v.StartedAt = null.TimeFrom(now)
	_, err := v.Update(ctx, c.db, boil.Whitelist(models.PodcastSegmentAnalysisColumns.Status, models.PodcastSegmentAnalysisColumns.StartedAt, models.PodcastSegmentAnalysisColumns.UpdatedAt))
	if err != nil {
		return PodcastSegmentAnalysis{}, false, err
	}
	return mapAnalysis(v, nil), true, nil
}
func (c *sqliteClient) CompletePodcastSegmentAnalysis(ctx context.Context, id, status, failure string, segments []PodcastSegment) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func(e error) error { _ = tx.Rollback(); return e }
	v, err := models.FindPodcastSegmentAnalysis(ctx, tx, id)
	if err != nil {
		return rollback(err)
	}
	now := time.Now()
	v.Status = status
	v.Error = null.StringFrom(failure)
	v.CompletedAt = null.TimeFrom(now)
	v.UpdatedAt = now
	if _, err = v.Update(ctx, tx, boil.Whitelist(models.PodcastSegmentAnalysisColumns.Status, models.PodcastSegmentAnalysisColumns.Error, models.PodcastSegmentAnalysisColumns.CompletedAt, models.PodcastSegmentAnalysisColumns.UpdatedAt)); err != nil {
		return rollback(err)
	}
	if _, err = models.PodcastEpisodeSegments(models.PodcastEpisodeSegmentWhere.AnalysisID.EQ(null.StringFrom(id))).DeleteAll(ctx, tx); err != nil {
		return rollback(err)
	}
	for i, s := range segments {
		row := &models.PodcastEpisodeSegment{ID: cuid.New(), VideoID: v.VideoID, AnalysisID: null.StringFrom(id), Source: PodcastSegmentSourceLLM, Position: int64(i), Category: s.Category, StartMS: int64(s.StartMS), EndMS: int64(s.EndMS), StartCue: int64(s.StartCue), EndCue: int64(s.EndCue), StartText: s.StartText, EndText: s.EndText, Reason: s.Reason, Brand: null.StringFrom(s.Brand)}
		if err = row.Insert(ctx, tx, boil.Infer()); err != nil {
			return rollback(err)
		}
	}
	return tx.Commit()
}
