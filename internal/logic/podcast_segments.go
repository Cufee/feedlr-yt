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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
)

const podcastSegmentsPromptVersion = "podcast-segments-v6"
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
	return "google/gemini-2.5-flash-lite"
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
	hash := hashTranscript(bytes, source.URL, failure, v.Description)
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
		go runPodcastSegmentAnalysis(db, a.ID, bytes, v.Description)
	}
	return PodcastSegmentStatus{Status: database.PodcastSegmentRunning}, nil
}

func hashTranscript(bytes []byte, url, failure, description string) string {
	h := sha256.New()
	h.Write(bytes)
	h.Write([]byte("\x00" + url + "\x00" + failure + "\x00" + description))
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

func runPodcastSegmentAnalysis(db database.Client, id string, data []byte, description string) {
	defer segmentRuns.Delete(id)
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cues, err := parseTimedTranscript(data)
	if err != nil {
		_ = db.CompletePodcastSegmentAnalysis(ctx, id, database.PodcastSegmentUnavailable, "transcript_parse_failed", nil)
		metrics.ObservePodcastSegmentAnalysis("transcript_parse_failed", time.Since(started).Seconds(), nil)
		return
	}
	notes := extractPodcastSponsors(ctx, description)
	segments, err := inferPodcastSegments(ctx, cues, notes)
	if err != nil {
		_ = db.CompletePodcastSegmentAnalysis(ctx, id, database.PodcastSegmentFailed, "model_output_invalid", nil)
		metrics.ObservePodcastSegmentAnalysis("model_output_invalid", time.Since(started).Seconds(), nil)
		return
	}
	_ = db.CompletePodcastSegmentAnalysis(ctx, id, database.PodcastSegmentReady, "", segments)
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
	result, err := openrouter.DefaultClient.CompleteWithOptions(extractionCtx, sponsorExtractionPrompt, notes, openrouter.CompletionOptions{MaxTokens: 400})
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

const segmentSystemPrompt = `Task: identify only clearly skippable segments in a podcast transcript. Allowed categories: sponsor (paid third-party advertisement, affiliate/referral, promo-code read), selfpromo (show/host/network promotion), interaction (brief request to like, subscribe, rate, follow, share, comment, or enable notifications), preview (a recap or preview whose substantive information is repeated later in the episode), intro (a narrated hook, greeting, or goodbye without information needed for the episode), filler (a clearly tangential joke or bit that is not needed to follow the main discussion).

Be conservative: return no segment unless it is a dedicated contiguous block. Do not infer from names alone or mark normal discussion, editorial mentions, internal funding, or dynamic ads absent from the transcript. A supplied show-notes sponsor list is only a clue: label a sponsor segment only when the transcript itself is a dedicated listener-facing ad read. In particular, do not mark a host's normal discussion of their own project, company, product infrastructure, funding, or roadmap merely because it describes benefits or uses promotional language. Do not mark a host reporting that their organization sponsors, funds, partners with, or supports a person, developer, company, or project; that is editorial discussion unless it becomes a dedicated listener-facing promotion. For preview, intro, and filler, use the category only when it is clearly dispensable; do not remove context, meaningful conclusions, or jokes that materially explain the discussion.

Boundary rule: once you identify an unambiguous host-read ad, return the complete contiguous block—not merely the cue that names the product. Ads may be performed as a skit: include the earliest consecutive fictional setup or dialogue that leads into the product mention, even where it does not name the product. Include every consecutive product claim, testimonial, feature list, scripted dialogue, promotion, or CTA, plus a closing character line or punchline that belongs to the ad even if it does not repeat the product name. End only at the first cue that clearly resumes the episode’s normal editorial discussion. Do not end the segment just because a short joke, character line, or transition occurs inside the same ad. Conversely, do not consume the first normal discussion cue after the ad has clearly ended.

Use supplied cue boundaries only. Never invent timestamps or cue indices. Return JSON {"segments":[{"category":"sponsor","start_cue":1,"end_cue":2,"brand":"","start_text":"","end_text":"","reason":""}]}.`

const segmentBoundaryPrompt = `A confirmed sponsor read has been found. Correct its boundaries using only the supplied cue records. Expand backward through the complete scripted problem setup or banter when it establishes the scenario that the product solves, even when the brand is introduced later. Expand forward through every product claim, fictional dialogue, punchline, or closing line belonging to the ad. Include an entire closing exchange, not only its first line. A final fictional callback to the ad's premise is still part of the ad even when it has no brand or feature language. Preserve meaningful editorial conversation before or after the ad, even if the ad starts or ends in the middle of a broader conversation. Return JSON {"start_cue":number,"end_cue":number}.`

const segmentBoundaryVerifierPrompt = `Review the boundaries of this already-confirmed sponsor read using only the supplied local cues. Check especially the cues immediately after its proposed end: if they are consecutive in-character dialogue, a callback to the ad's problem scenario, or the final beat of the same scripted read, extend through the entire exchange. Do not retain a product-claim boundary when a clearly connected closing line follows. Do not include normal editorial conversation. Return JSON {"start_cue":number,"end_cue":number}.`

func inferPodcastSegments(ctx context.Context, cues []transcriptCue, notes string) ([]database.PodcastSegment, error) {
	if compactCueSize(cues) <= 1_500_000 {
		return inferCueWindow(ctx, cues, cues, 0, len(cues)-1, notes)
	}
	var all []database.PodcastSegment
	for start := 0; start < len(cues); {
		end, startSize := start, 0
		for end < len(cues) && startSize+len(cues[end].Text)+64 <= 300_000 {
			startSize += len(cues[end].Text) + 64
			end++
		}
		if end == start {
			end++
		}
		from, to := max(0, start-20), min(len(cues), end+20)
		segments, err := inferCueWindow(ctx, cues, cues[from:to], cues[start].Index, cues[end-1].Index, notes)
		if err != nil {
			return nil, err
		}
		all = append(all, segments...)
		start = end
	}
	return mergePodcastSegments(all), nil
}
func compactCueSize(cues []transcriptCue) int {
	size := 0
	for _, cue := range cues {
		size += len(cue.Text) + 64
	}
	return size
}
func inferCueWindow(ctx context.Context, all, window []transcriptCue, targetStart, targetEnd int, notes string) ([]database.PodcastSegment, error) {
	payload := make([]map[string]any, len(window))
	for i, c := range window {
		payload[i] = map[string]any{"i": c.Index, "start_ms": c.StartMS, "end_ms": c.EndMS, "text": c.Text}
	}
	inputPayload := map[string]any{"cues": payload}
	if notes != "" {
		var sponsors []string
		if json.Unmarshal([]byte(notes), &sponsors) == nil {
			inputPayload["show_notes_explicit_sponsors"] = sponsors
		}
	}
	encoded, _ := json.Marshal(inputPayload)
	input := string(encoded)
	if targetStart != 0 || targetEnd != len(all)-1 {
		input += fmt.Sprintf("\nReport only segments intersecting target cue range %d through %d.", targetStart, targetEnd)
	}
	result, err := openrouter.DefaultClient.Complete(ctx, segmentSystemPrompt, input)
	if err != nil {
		return nil, err
	}
	var out struct {
		Segments []struct {
			Category  string `json:"category"`
			StartCue  int    `json:"start_cue"`
			EndCue    int    `json:"end_cue"`
			Brand     string `json:"brand"`
			StartText string `json:"start_text"`
			EndText   string `json:"end_text"`
			Reason    string `json:"reason"`
		} `json:"segments"`
	}
	if err := json.Unmarshal([]byte(result.Content), &out); err != nil {
		return nil, err
	}
	byIndex := make(map[int]transcriptCue, len(all))
	for _, cue := range all {
		byIndex[cue.Index] = cue
	}
	var segments []database.PodcastSegment
	for _, candidate := range out.Segments {
		start, ok := byIndex[candidate.StartCue]
		end, endOK := byIndex[candidate.EndCue]
		if !ok || !endOK || candidate.EndCue < candidate.StartCue || !validCategory(candidate.Category) {
			return nil, errors.New("invalid segment")
		}
		if candidate.EndCue < targetStart || candidate.StartCue > targetEnd {
			continue
		}
		if end.EndMS-start.StartMS < 1500 && candidate.Category != "interaction" {
			continue
		}
		if strings.TrimSpace(candidate.Reason) == "" {
			return nil, errors.New("missing evidence")
		}
		segments = append(segments, database.PodcastSegment{Category: candidate.Category, StartMS: start.StartMS, EndMS: end.EndMS, StartCue: start.Index, EndCue: end.Index, StartText: start.Text, EndText: end.Text, Reason: strings.TrimSpace(candidate.Reason), Brand: strings.TrimSpace(candidate.Brand)})
	}
	return refineSponsorBoundaries(ctx, all, mergePodcastSegments(segments)), nil
}

func refineSponsorBoundaries(ctx context.Context, cues []transcriptCue, segments []database.PodcastSegment) []database.PodcastSegment {
	byIndex := make(map[int]int, len(cues))
	for offset, cue := range cues {
		byIndex[cue.Index] = offset
	}
	for i := range segments {
		segment := &segments[i]
		if segment.Category != "sponsor" || segment.Brand == "" || !strings.Contains(strings.ToLower(segment.StartText), strings.ToLower(segment.Brand)) {
			continue
		}
		start, foundStart := byIndex[segment.StartCue]
		end, foundEnd := byIndex[segment.EndCue]
		if !foundStart || !foundEnd {
			continue
		}
		from, to := max(0, start-20), min(len(cues), end+21)
		payloadCues := make([]map[string]any, 0, to-from)
		for _, cue := range cues[from:to] {
			payloadCues = append(payloadCues, map[string]any{"i": cue.Index, "start_ms": cue.StartMS, "end_ms": cue.EndMS, "text": cue.Text})
		}
		input, err := json.Marshal(map[string]any{"candidate": map[string]any{"category": segment.Category, "start_cue": segment.StartCue, "end_cue": segment.EndCue, "brand": segment.Brand}, "cues": payloadCues})
		if err != nil {
			continue
		}
		for _, prompt := range []string{segmentBoundaryPrompt, segmentBoundaryVerifierPrompt} {
			result, err := openrouter.DefaultClient.Complete(ctx, prompt, string(input))
			if err != nil {
				continue
			}
			var refined struct {
				StartCue int `json:"start_cue"`
				EndCue   int `json:"end_cue"`
			}
			if err := json.Unmarshal([]byte(result.Content), &refined); err != nil {
				continue
			}
			refinedStart, validStart := byIndex[refined.StartCue]
			refinedEnd, validEnd := byIndex[refined.EndCue]
			if !validStart || !validEnd || refinedStart < from || refinedEnd >= to || refinedEnd < refinedStart {
				continue
			}
			first, last := cues[refinedStart], cues[refinedEnd]
			segment.StartCue, segment.EndCue = first.Index, last.Index
			segment.StartMS, segment.EndMS = first.StartMS, last.EndMS
			segment.StartText, segment.EndText = first.Text, last.Text
		}
	}
	return mergePodcastSegments(segments)
}
func mergePodcastSegments(segments []database.PodcastSegment) []database.PodcastSegment {
	sort.Slice(segments, func(i, j int) bool { return segments[i].StartMS < segments[j].StartMS })
	dedup := segments[:0]
	for _, s := range segments {
		if len(dedup) > 0 && dedup[len(dedup)-1].Category == s.Category && s.StartMS <= dedup[len(dedup)-1].EndMS {
			if s.EndMS > dedup[len(dedup)-1].EndMS {
				dedup[len(dedup)-1].EndMS = s.EndMS
			}
			continue
		}
		dedup = append(dedup, s)
	}
	return dedup
}
func validCategory(c string) bool {
	return c == "sponsor" || c == "selfpromo" || c == "interaction" || c == "preview" || c == "intro" || c == "filler"
}
func FormatPodcastSegmentTime(ms int) string { return fmt.Sprintf("%d:%02d", ms/60000, (ms/1000)%60) }
