package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
)

const podcastSegmentsPromptVersion = "podcast-segments-v7"
const podcastSegmentLease = 10 * time.Minute

type PodcastSegmentStatus struct {
	Status   string
	Segments []database.PodcastSegment
}
type transcriptCue struct {
	Index, StartMS, EndMS int
	Text                  string
}

var segmentRuns sync.Map

func podcastModel() string {
	if openrouter.DefaultClient != nil {
		return openrouter.DefaultClient.Model()
	}
	return openrouter.DefaultModel
}

// EnsurePodcastSegmentAnalysis obtains durable state and starts at most one
// local goroutine. A duplicate process joins through the database uniqueness key.
func EnsurePodcastSegmentAnalysis(ctx context.Context, db database.Client, videoID string) (PodcastSegmentStatus, error) {
	v, err := db.GetVideoByID(ctx, videoID)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	if v.Type != string(youtube.VideoTypePodcastEpisode) {
		return PodcastSegmentStatus{}, errors.New("video is not a podcast episode")
	}
	source, err := db.GetPodcastTranscript(ctx, videoID)
	if err != nil {
		return PodcastSegmentStatus{Status: database.PodcastSegmentUnavailable}, nil
	}
	if openrouter.DefaultClient == nil {
		return PodcastSegmentStatus{Status: database.PodcastSegmentUnavailable}, nil
	}
	bytes, failure := fetchTranscript(ctx, source.URL, source.MIMEType)
	hash := hashTranscript(bytes, source.URL, failure, v.Description, v.Title)
	model := podcastModel()
	a, owner, err := db.AcquirePodcastSegmentAnalysis(ctx, videoID, hash, source.URL, model, podcastSegmentsPromptVersion)
	if err != nil {
		return PodcastSegmentStatus{}, err
	}
	if a.Status == database.PodcastSegmentRunning && a.StartedAt.Valid && time.Since(a.StartedAt.Time) > podcastSegmentLease {
		_ = db.CompletePodcastSegmentAnalysis(context.Background(), a.ID, database.PodcastSegmentFailed, "analysis_interrupted", nil)
		a.Status = database.PodcastSegmentFailed
	}
	if !owner {
		return PodcastSegmentStatus{Status: a.Status, Segments: a.Segments}, nil
	}
	if failure != "" {
		_ = db.CompletePodcastSegmentAnalysis(context.Background(), a.ID, database.PodcastSegmentUnavailable, failure, nil)
		return PodcastSegmentStatus{Status: database.PodcastSegmentUnavailable}, nil
	}
	if _, loaded := segmentRuns.LoadOrStore(a.ID, struct{}{}); !loaded {
		go runPodcastSegmentAnalysis(db, a.ID, bytes, v.Description, v.Title)
	}
	return PodcastSegmentStatus{Status: database.PodcastSegmentRunning}, nil
}

func hashTranscript(bytes []byte, url, failure, description, title string) string {
	h := sha256.New()
	h.Write(bytes)
	h.Write([]byte("\x00" + url + "\x00" + failure + "\x00" + description + "\x00" + title))
	return hex.EncodeToString(h.Sum(nil))
}
func fetchTranscript(ctx context.Context, url, mime string) ([]byte, string) {
	if !isTimedTranscript(mime) {
		return nil, "unsupported_transcript"
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "transcript_fetch_failed"
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "transcript_fetch_failed"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "transcript_fetch_failed"
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || len(data) == 8<<20 {
		return nil, "transcript_fetch_failed"
	}
	return data, ""
}
func isTimedTranscript(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0])) {
	case "text/vtt", "application/x-subrip", "application/srt":
		return true
	}
	return false
}

func runPodcastSegmentAnalysis(db database.Client, id string, data []byte, description, title string) {
	defer segmentRuns.Delete(id)
	started := time.Now()
	complete := func(status, failure string, segments []database.PodcastSegment) {
		completionCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.CompletePodcastSegmentAnalysis(completionCtx, id, status, failure, segments)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cues, err := parseTimedTranscript(data)
	if err != nil {
		complete(database.PodcastSegmentUnavailable, "transcript_parse_failed", nil)
		metrics.ObservePodcastSegmentAnalysis("transcript_parse_failed", time.Since(started).Seconds(), nil)
		return
	}
	notes := extractPodcastSponsors(ctx, description)
	segments, err := inferPodcastSegments(ctx, cues, notes, title)
	if err != nil {
		complete(database.PodcastSegmentFailed, "model_output_invalid", nil)
		metrics.ObservePodcastSegmentAnalysis("model_output_invalid", time.Since(started).Seconds(), nil)
		return
	}
	complete(database.PodcastSegmentReady, "", segments)
	categories := make([]string, 0, len(segments))
	for _, segment := range segments {
		categories = append(categories, segment.Category)
	}
	metrics.ObservePodcastSegmentAnalysis("ready", time.Since(started).Seconds(), categories)
}

func parseTimedTranscript(data []byte) ([]transcriptCue, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	blocks := strings.Split(text, "\n\n")
	var cues []transcriptCue
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) == 0 || strings.HasPrefix(strings.ToUpper(lines[0]), "WEBVTT") {
			continue
		}
		timeLine := 0
		for i, line := range lines {
			if strings.Contains(line, "-->") {
				timeLine = i
				break
			}
		}
		if timeLine == 0 && !strings.Contains(lines[0], "-->") {
			continue
		}
		parts := strings.Split(lines[timeLine], "-->")
		if len(parts) != 2 {
			return nil, errors.New("invalid cue")
		}
		start, ok := parseCueTime(parts[0])
		if !ok {
			return nil, errors.New("invalid cue start")
		}
		end, ok := parseCueTime(strings.Fields(parts[1])[0])
		if !ok || end <= start {
			return nil, errors.New("invalid cue end")
		}
		body := cleanCueText(strings.Join(lines[timeLine+1:], " "))
		if body == "" {
			continue
		}
		cues = append(cues, transcriptCue{Index: len(cues), StartMS: start, EndMS: end, Text: body})
	}
	if len(cues) == 0 {
		return nil, errors.New("no cues")
	}
	return cues, nil
}
func parseCueTime(raw string) (int, bool) {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	sec := parts[len(parts)-1]
	sp := strings.Split(sec, ".")
	if len(sp) > 2 {
		return 0, false
	}
	s, e := strconv.Atoi(sp[0])
	if e != nil || s < 0 || s > 59 {
		return 0, false
	}
	ms := 0
	if len(sp) == 2 {
		frac := sp[1] + "000"
		ms, e = strconv.Atoi(frac[:3])
		if e != nil {
			return 0, false
		}
	}
	m, e := strconv.Atoi(parts[len(parts)-2])
	if e != nil || m < 0 || m > 59 {
		return 0, false
	}
	h := 0
	if len(parts) == 3 {
		h, e = strconv.Atoi(parts[0])
		if e != nil || h < 0 {
			return 0, false
		}
	}
	return ((h*3600 + m*60 + s) * 1000) + ms, true
}

func cleanCueText(s string) string {
	var text strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
			text.WriteByte(' ')
		case '>':
			inTag = false
			text.WriteByte(' ')
		default:
			if !inTag {
				text.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(text.String())), " ")
}

const sponsorExtractionPrompt = `Extract only explicitly named, paid third-party sponsors or advertisers from these podcast episode notes. Return JSON exactly as {"sponsors":["brand name"]}. Do not infer sponsors from ordinary links, guests, recommended products, donations, self-promotion, or phrases such as "support the show". Return an empty array when the notes do not explicitly identify a paid sponsor.`

// extractPodcastSponsors uses a narrow AI pass because show-note conventions
// vary too much for reliable pattern matching. Its output is a hint only; the
// transcript pass still needs an actual listener-facing ad read to emit a skip.
func extractPodcastSponsors(ctx context.Context, description string) string {
	notes := cleanCueText(description)
	if notes == "" || openrouter.DefaultClient == nil {
		return ""
	}
	if len(notes) > 24_000 {
		notes = notes[:24_000]
	}
	extractionCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := openrouter.DefaultClient.CompleteWithOptions(extractionCtx, sponsorExtractionPrompt, notes, openrouter.CompletionOptions{MaxTokens: 2048, ReasoningEffort: "low"})
	if err != nil {
		return ""
	}
	var output struct {
		Sponsors []string `json:"sponsors"`
	}
	if json.Unmarshal([]byte(result.Content), &output) != nil {
		return ""
	}
	seen := make(map[string]struct{}, len(output.Sponsors))
	sponsors := make([]string, 0, len(output.Sponsors))
	for _, sponsor := range output.Sponsors {
		sponsor = strings.TrimSpace(sponsor)
		if sponsor == "" || len(sponsor) > 160 {
			continue
		}
		key := strings.ToLower(sponsor)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		sponsors = append(sponsors, sponsor)
		if len(sponsors) == 12 {
			break
		}
	}
	if len(sponsors) == 0 {
		return ""
	}
	encoded, _ := json.Marshal(sponsors)
	return string(encoded)
}

func validCategory(c string) bool {
	return c == "sponsor" || c == "selfpromo" || c == "interaction" || c == "preview" || c == "intro" || c == "filler"
}
func FormatPodcastSegmentTime(ms int) string { return fmt.Sprintf("%d:%02d", ms/60000, (ms/1000)%60) }
