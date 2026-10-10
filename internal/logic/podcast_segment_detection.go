package logic

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"golang.org/x/sync/errgroup"
)

const (
	segmentCoreMS          = 120_000
	segmentPaddingMS       = 30_000
	segmentExpansionMS     = 120_000
	segmentExpansionRounds = 3
	segmentWorkers         = 4
	segmentPack            = 4
	segmentMaxCalls        = 256
)

type segmentCompleter interface {
	CompleteWithOptions(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error)
}

type segmentDetector struct {
	client       segmentCompleter
	cues         []transcriptCue
	title, notes string
	calls        atomic.Int32
}

type segmentWindow struct{ id, coreStart, coreEnd, from, to int }
type segmentPoint struct{ cue, offset int }
type groundedSegment struct {
	start, end              segmentPoint
	category, brand, reason string
}

type segmentAnswer struct {
	Category   string `json:"category"`
	StartCue   *int   `json:"start_cue"`
	EndCue     *int   `json:"end_cue"`
	StartQuote string `json:"start_quote"`
	EndQuote   string `json:"end_quote"`
	Brand      string `json:"brand"`
	Reason     string `json:"reason"`
	Evidence   []struct {
		CueID *int   `json:"cue_id"`
		Quote string `json:"quote"`
	} `json:"evidence"`
}

type segmentWindowAnswer struct {
	ID        *int             `json:"window_id"`
	Reviewed  bool             `json:"reviewed"`
	NeedLeft  *bool            `json:"need_left"`
	NeedRight *bool            `json:"need_right"`
	Segments  *[]segmentAnswer `json:"segments"`
}

func inferPodcastSegments(ctx context.Context, cues []transcriptCue, notes, title string) ([]database.PodcastSegment, error) {
	if openrouter.DefaultClient == nil {
		return nil, errors.New("segment provider unavailable")
	}
	d := &segmentDetector{client: openrouter.DefaultClient, cues: cues, title: title, notes: notes}
	return d.detect(ctx)
}

func (d *segmentDetector) window(start, end, padding int) segmentWindow {
	w := segmentWindow{id: start, coreStart: start, coreEnd: end, from: start, to: end}
	return d.expand(w, padding, padding)
}

func (d *segmentDetector) expand(w segmentWindow, left, right int) segmentWindow {
	startMS, endMS := d.cues[w.from].StartMS-left, d.cues[w.to].EndMS+right
	for left > 0 && w.from > 0 && d.cues[w.from-1].EndMS > startMS {
		w.from--
	}
	for right > 0 && w.to+1 < len(d.cues) && d.cues[w.to+1].StartMS < endMS {
		w.to++
	}
	return w
}

func (d *segmentDetector) windows() []segmentWindow {
	var windows []segmentWindow
	for start := 0; start < len(d.cues); {
		end, size := start, len(d.cues[start].Text)
		// The byte bound also handles dense or malformed publisher timestamps.
		for end+1 < len(d.cues) && d.cues[end+1].StartMS < d.cues[start].StartMS+segmentCoreMS && size+len(d.cues[end+1].Text) < 40_000 {
			end++
			size += len(d.cues[end].Text)
		}
		windows = append(windows, d.window(start, end, segmentPaddingMS))
		start = end + 1
	}
	return windows
}

func (d *segmentDetector) detect(ctx context.Context) ([]database.PodcastSegment, error) {
	if len(d.cues) == 0 {
		return nil, errors.New("no transcript cues")
	}
	windows := d.windows()
	batches := make([][]groundedSegment, (len(windows)+segmentPack-1)/segmentPack)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(segmentWorkers)
	for batch := range batches {
		group.Go(func() error {
			pack := windows[batch*segmentPack : min(len(windows), (batch+1)*segmentPack)]
			segments, err := d.review(groupCtx, pack, segmentDiscoveryPrompt, "medium", false, 0)
			if err != nil && groupCtx.Err() == nil {
				// An invalid discovery response is never an ad-free verdict. Give
				// each source window a bounded independent review before failing.
				segments = nil
				for _, w := range pack {
					var recovered []groundedSegment
					recovered, err = d.review(groupCtx, []segmentWindow{w}, segmentDiscoveryPrompt, "high", false, 0)
					if err != nil {
						return err
					}
					segments = append(segments, recovered...)
				}
			}
			batches[batch] = segments
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	var proposals []groundedSegment
	var skips []database.PodcastSegment
	for _, batch := range batches {
		for _, s := range batch {
			if s.category == "sponsor" {
				proposals = append(proposals, s)
			} else {
				skips = append(skips, d.interior(s)...)
			}
		}
	}
	// Coalesce overlapping candidate regions for review, but retain each
	// grounded span for playback support. Never fill gaps between proposals.
	regions := sponsorRegions(proposals)
	confirmed := make([][]database.PodcastSegment, len(regions))
	group, groupCtx = errgroup.WithContext(ctx)
	group.SetLimit(segmentWorkers)
	for i, region := range regions {
		group.Go(func() error {
			finals, err := d.review(groupCtx, []segmentWindow{d.window(region[0], region[1], segmentExpansionMS)}, segmentConfirmationPrompt, "high", true, 0)
			if err != nil {
				return err
			}
			for _, final := range finals {
				supported := d.supportedInteriors(final, proposals)
				if segmentDuration(supported) < segmentDuration(d.interior(final)) {
					// Review only when the support guard clips otherwise skippable
					// ad speech. Mixed edge cues stay audible in either outcome.
					boundaries, err := d.review(groupCtx, []segmentWindow{d.window(final.start.cue, final.end.cue, segmentExpansionMS)}, segmentBoundaryReviewPrompt, "high", true, 0)
					if err == nil {
						supported = d.supportedInteriors(final, boundaries)
					}
					// Failure retains the conservative skip; a valid rejection
					// (empty boundaries) removes the skip entirely.
				}
				confirmed[i] = append(confirmed[i], supported...)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	for _, batch := range confirmed {
		skips = append(skips, batch...)
	}
	return mergePodcastSegments(skips), nil
}

func (d *segmentDetector) review(ctx context.Context, windows []segmentWindow, prompt, effort string, sponsorOnly bool, round int) ([]groundedSegment, error) {
	payloadWindows := make([]map[string]any, len(windows))
	for i, w := range windows {
		cues := make([]map[string]any, 0, w.to-w.from+1)
		for _, c := range d.cues[w.from : w.to+1] {
			cues = append(cues, map[string]any{"i": c.Index, "text": c.Text})
		}
		payloadWindows[i] = map[string]any{"window_id": w.id, "core": [2]int{w.coreStart, w.coreEnd}, "cues": cues}
	}
	payload := map[string]any{"episode_title": d.title, "windows": payloadWindows}
	if !sponsorOnly && d.notes != "" {
		var sponsors []string
		if json.Unmarshal([]byte(d.notes), &sponsors) == nil {
			payload["show_notes_explicit_sponsors"] = sponsors
		}
	}
	var answers []segmentWindowAnswer
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if d.calls.Add(1) > segmentMaxCalls {
			return nil, errors.New("segment request limit reached")
		}
		encoded, _ := json.Marshal(payload)
		if len(encoded) > 300_000 {
			return nil, errors.New("segment context too large")
		}
		result, callErr := d.client.CompleteWithOptions(ctx, prompt+segmentResponsePrompt, string(encoded), openrouter.CompletionOptions{MaxTokens: 16_384, ReasoningEffort: effort})
		metrics.ObservePodcastProviderCost("sponsor_scanning", result.Usage.Cost)
		if callErr != nil {
			return nil, callErr
		}
		answers, err = d.decode(result.Content, windows, sponsorOnly)
		if err == nil {
			break
		}
		payload["validation_feedback"] = err.Error()
		payload["invalid_answer"] = result.Content
	}
	if err != nil {
		return nil, err
	}
	var segments []groundedSegment
	for i, answer := range answers {
		w := windows[i]
		if *answer.NeedLeft || *answer.NeedRight {
			if round >= segmentExpansionRounds {
				return nil, errors.New("segment context remains unresolved")
			}
			left, right := 0, 0
			if *answer.NeedLeft {
				left = segmentExpansionMS
			}
			if *answer.NeedRight {
				right = segmentExpansionMS
			}
			expanded := d.expand(w, left, right)
			if expanded.from == w.from && expanded.to == w.to {
				return nil, errors.New("segment context unavailable")
			}
			more, err := d.review(ctx, []segmentWindow{expanded}, prompt, effort, sponsorOnly, round+1)
			if err != nil {
				return nil, err
			}
			segments = append(segments, more...)
			continue
		}
		for _, s := range *answer.Segments {
			grounded, err := d.ground(s, w, sponsorOnly)
			if err != nil {
				return nil, err
			}
			segments = append(segments, grounded)
		}
	}
	return segments, nil
}

func (d *segmentDetector) decode(content string, windows []segmentWindow, sponsorOnly bool) ([]segmentWindowAnswer, error) {
	var output struct {
		Windows []segmentWindowAnswer `json:"windows"`
	}
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, errors.New("invalid segment JSON")
	}
	if len(output.Windows) != len(windows) {
		return nil, errors.New("missing window decisions")
	}
	byID := make(map[int]segmentWindowAnswer, len(windows))
	for _, answer := range output.Windows {
		if answer.ID == nil || !answer.Reviewed || answer.NeedLeft == nil || answer.NeedRight == nil || answer.Segments == nil {
			return nil, errors.New("incomplete window decision")
		}
		if _, exists := byID[*answer.ID]; exists {
			return nil, errors.New("duplicate window decision")
		}
		byID[*answer.ID] = answer
	}
	answers := make([]segmentWindowAnswer, len(windows))
	for i, w := range windows {
		answer, exists := byID[w.id]
		if !exists {
			return nil, errors.New("missing window decision")
		}
		// Do not consume partial answers while additional context is requested.
		if !*answer.NeedLeft && !*answer.NeedRight {
			for _, s := range *answer.Segments {
				if _, err := d.ground(s, w, sponsorOnly); err != nil {
					return nil, err
				}
			}
		}
		answers[i] = answer
	}
	return answers, nil
}

func quoteOffset(text, quote string) (int, error) {
	if quote == "" || strings.TrimSpace(quote) != quote {
		return 0, errors.New("missing exact quote")
	}
	i := strings.Index(text, quote)
	if i < 0 || strings.Contains(text[i+1:], quote) {
		return 0, errors.New("quote must uniquely match its cue")
	}
	return i, nil
}

func (d *segmentDetector) ground(s segmentAnswer, w segmentWindow, sponsorOnly bool) (groundedSegment, error) {
	invalid := func() (groundedSegment, error) { return groundedSegment{}, errors.New("invalid segment or evidence") }
	if s.StartCue == nil || s.EndCue == nil || !validCategory(s.Category) || (sponsorOnly && s.Category != "sponsor") || strings.TrimSpace(s.Reason) == "" {
		return invalid()
	}
	a, b := *s.StartCue, *s.EndCue
	if a < w.from || b > w.to || b < a || a > w.coreEnd || b < w.coreStart {
		return invalid()
	}
	start, err := quoteOffset(d.cues[a].Text, s.StartQuote)
	if err != nil {
		return groundedSegment{}, err
	}
	end, err := quoteOffset(d.cues[b].Text, s.EndQuote)
	if err != nil {
		return groundedSegment{}, err
	}
	g := groundedSegment{start: segmentPoint{a, start}, end: segmentPoint{b, end + len(s.EndQuote)}, category: s.Category, brand: strings.TrimSpace(s.Brand), reason: strings.TrimSpace(s.Reason)}
	if !pointBefore(g.start, g.end) {
		return invalid()
	}
	if s.Category == "sponsor" && (len(s.Evidence) == 0 || len(s.Evidence) > 2) {
		return invalid()
	}
	for _, e := range s.Evidence {
		if e.CueID == nil || *e.CueID < a || *e.CueID > b {
			return invalid()
		}
		cue := d.cues[*e.CueID]
		offset, err := quoteOffset(cue.Text, e.Quote)
		if err != nil {
			return groundedSegment{}, err
		}
		if e.Quote != cue.Text && (len([]rune(e.Quote)) < 12 || len(strings.Fields(e.Quote)) < 3) {
			return invalid()
		}
		if pointBefore(segmentPoint{*e.CueID, offset}, g.start) || pointBefore(g.end, segmentPoint{*e.CueID, offset + len(e.Quote)}) {
			return invalid()
		}
	}
	return g, nil
}

func pointBefore(a, b segmentPoint) bool {
	return a.cue < b.cue || (a.cue == b.cue && a.offset < b.offset)
}
func substantive(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) })
}

func (d *segmentDetector) interior(s groundedSegment) []database.PodcastSegment {
	a, b := s.start.cue, s.end.cue
	if substantive(d.cues[a].Text[:s.start.offset]) {
		a++
	}
	if substantive(d.cues[b].Text[s.end.offset:]) {
		b--
	}
	if a > b {
		return nil
	}
	first, last := d.cues[a], d.cues[b]
	if last.EndMS <= first.StartMS || (last.EndMS-first.StartMS < 1500 && s.category != "interaction") {
		return nil
	}
	return []database.PodcastSegment{{Category: s.category, Brand: s.brand, Reason: s.reason, StartMS: first.StartMS, EndMS: last.EndMS, StartCue: first.Index, EndCue: last.Index, StartText: first.Text, EndText: last.Text}}
}

func (d *segmentDetector) supportedInteriors(final groundedSegment, proposals []groundedSegment) []database.PodcastSegment {
	var spans []groundedSegment
	for _, p := range proposals {
		if p.category != final.category || (p.brand != "" && final.brand != "" && !strings.EqualFold(p.brand, final.brand)) {
			continue
		}
		span := final
		if pointBefore(span.start, p.start) {
			span.start = p.start
		}
		if pointBefore(p.end, span.end) {
			span.end = p.end
		}
		if pointBefore(span.start, span.end) {
			spans = append(spans, span)
		}
	}
	sort.Slice(spans, func(i, j int) bool { return pointBefore(spans[i].start, spans[j].start) })
	var merged []groundedSegment
	for _, s := range spans {
		if len(merged) > 0 && !pointBefore(merged[len(merged)-1].end, s.start) {
			last := &merged[len(merged)-1]
			if pointBefore(last.end, s.end) {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}
	var skips []database.PodcastSegment
	for _, s := range merged {
		skips = append(skips, d.interior(s)...)
	}
	return mergePodcastSegments(skips)
}

func sponsorRegions(proposals []groundedSegment) [][2]int {
	var regions [][2]int
	for _, s := range proposals {
		regions = append(regions, [2]int{s.start.cue, s.end.cue})
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i][0] < regions[j][0] })
	merged := regions[:0]
	for _, r := range regions {
		if len(merged) > 0 && r[0] <= merged[len(merged)-1][1] {
			merged[len(merged)-1][1] = max(merged[len(merged)-1][1], r[1])
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

func segmentDuration(segments []database.PodcastSegment) int {
	total := 0
	for _, s := range segments {
		total += s.EndMS - s.StartMS
	}
	return total
}

func mergePodcastSegments(segments []database.PodcastSegment) []database.PodcastSegment {
	// Merge within categories first so an interleaved category cannot prevent
	// deduplication. Never merge across an audible gap or different advertisers.
	sort.Slice(segments, func(i, j int) bool {
		a, b := segments[i], segments[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.Brand != b.Brand {
			return a.Brand < b.Brand
		}
		return a.StartMS < b.StartMS
	})
	merged := segments[:0]
	for _, s := range segments {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if last.Category == s.Category && last.Brand == s.Brand && s.StartMS <= last.EndMS {
				if s.EndMS > last.EndMS {
					last.EndMS, last.EndCue, last.EndText = s.EndMS, s.EndCue, s.EndText
				}
				continue
			}
		}
		merged = append(merged, s)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].StartMS < merged[j].StartMS })
	return merged
}
