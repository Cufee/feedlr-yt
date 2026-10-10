package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
)

func progressiveTestDetector(cues []transcriptCue, core, first, last, total int) segmentDetector {
	input := &podcastScanInput{Cues: cues, CoreIndex: core, FirstChunk: first, LastChunk: last, TotalChunks: total, DurationMS: total * podcastChunkMS}
	return segmentDetector{cues: cues, progressive: input}
}

func progressiveTestResponse(input string, cues []transcriptCue, candidates ...[2]int) (openrouter.Result, error) {
	segments := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		segments = append(segments, segmentTestAnswer(cues, candidate[0], candidate[1], "sponsor"))
	}
	return progressiveTestResponseWithSegments(input, segments...)
}

type progressiveTestRequest struct {
	Windows []struct {
		ID   int    `json:"window_id"`
		Core [2]int `json:"core"`
	} `json:"windows"`
}

func progressiveTestResponseWithSegments(input string, candidates ...map[string]any) (openrouter.Result, error) {
	var payload progressiveTestRequest
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return openrouter.Result{}, err
	}
	var answers []any
	for _, window := range payload.Windows {
		segments := []map[string]any{}
		for _, candidate := range candidates {
			if candidate["start_cue"].(int) <= window.Core[1] && candidate["end_cue"].(int) >= window.Core[0] {
				segments = append(segments, candidate)
			}
		}
		answers = append(answers, map[string]any{"window_id": window.ID, "reviewed": true, "need_left": false, "need_right": false, "segments": segments})
	}
	body, err := json.Marshal(map[string]any{"windows": answers})
	return openrouter.Result{Content: string(body)}, err
}

func TestProgressiveDetectionWindowsCoverOnlyOwnedCore(t *testing.T) {
	texts := make([]string, 180)
	for i := range texts {
		texts[i] = fmt.Sprintf("Editorial sentence %d.", i)
	}
	cues := segmentTestCues(texts...)
	d := progressiveTestDetector(cues, 1, 0, 2, 3)
	coverage := make([]int, len(cues))
	paddingOutsideCore := false
	for _, window := range d.windows() {
		for i := window.coreStart; i <= window.coreEnd; i++ {
			coverage[i]++
		}
		paddingOutsideCore = paddingOutsideCore || !d.progressive.owns(cues[window.from]) || !d.progressive.owns(cues[window.to])
	}
	for i, cue := range cues {
		midpoint := cue.StartMS + (cue.EndMS-cue.StartMS)/2
		want := 0
		if midpoint >= podcastChunkMS && midpoint < 2*podcastChunkMS {
			want = 1
		}
		if coverage[i] != want {
			t.Fatalf("cue %d core coverage = %d, want %d", i, coverage[i], want)
		}
	}
	if !paddingOutsideCore {
		t.Fatal("owned core lost neighboring context padding")
	}
	// Crossing cues have exactly one owner even when their start precedes a core.
	crossing := transcriptCue{StartMS: podcastChunkMS - 1000, EndMS: podcastChunkMS + 3000}
	left := *d.progressive
	left.CoreIndex = 0
	if left.owns(crossing) || !d.progressive.owns(crossing) {
		t.Fatal("cross-core cue midpoint did not assign exactly one owner")
	}
}

func TestProgressiveDetectionSilentCoreDoesNotScanNeighborAds(t *testing.T) {
	cues := segmentTestCues("Buy Acme now.", "Visit Acme today.")
	cues[1].StartMS, cues[1].EndMS = 2*podcastChunkMS, 2*podcastChunkMS+10000
	d := progressiveTestDetector(cues, 1, 0, 2, 3)
	var calls atomic.Int32
	d.client = segmentCompleteFunc(func(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error) {
		calls.Add(1)
		return openrouter.Result{}, errors.New("silent core should not request discovery")
	})
	got, err := d.detect(context.Background())
	if err != nil || len(got) != 0 || calls.Load() != 0 {
		t.Fatalf("neighboring speech was treated as silent core content: %+v, %v, calls=%d", got, err, calls.Load())
	}
}

func TestProgressiveDetectionTemporaryEdgesDeferDespiteModelFlags(t *testing.T) {
	for _, tc := range []struct {
		name        string
		start, end  int
		left, right bool
	}{
		{"left edge", 0, 1, true, false},
		{"right edge", 1, 2, false, true},
		{"both edges", 0, 2, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cues := segmentTestCues("Buy Acme storage.", "Acme protects your files.", "Visit Acme today.")
			for i := range cues {
				cues[i].StartMS += podcastChunkMS
				cues[i].EndMS += podcastChunkMS
			}
			d := progressiveTestDetector(cues, 1, 1, 1, 3)
			var calls, callbacks atomic.Int32
			d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				calls.Add(1)
				if !strings.Contains(input, `"context_start_is_episode_start":false`) || !strings.Contains(input, `"context_end_is_episode_end":false`) {
					return openrouter.Result{}, errors.New("temporary context edges presented as episode ends")
				}
				return progressiveTestResponse(input, cues, [2]int{tc.start, tc.end})
			})
			d.confirmed = func([]database.PodcastSegment, groundedSegment) error { callbacks.Add(1); return nil }
			got, err := d.detect(context.Background())
			var deferred *podcastContextDeferred
			if !errors.As(err, &deferred) || deferred.Left != tc.left || deferred.Right != tc.right || len(got) != 0 || calls.Load() != 1 || callbacks.Load() != 0 {
				t.Fatalf("temporary boundary was finalized or retried: %+v, %v, calls=%d callbacks=%d", got, err, calls.Load(), callbacks.Load())
			}
		})
	}
}

func TestProgressiveDetectionMissingRequestedContextDefers(t *testing.T) {
	for _, tc := range []struct{ left, right bool }{{true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("left=%v/right=%v", tc.left, tc.right), func(t *testing.T) {
			cues := segmentTestCues("An unresolved commercial.")
			cues[0].StartMS, cues[0].EndMS = podcastChunkMS, podcastChunkMS+10000
			d := progressiveTestDetector(cues, 1, 1, 1, 3)
			var calls atomic.Int32
			d.client = segmentCompleteFunc(func(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error) {
				calls.Add(1)
				body, _ := json.Marshal(map[string]any{"windows": []any{map[string]any{"window_id": 0, "reviewed": true, "need_left": tc.left, "need_right": tc.right, "segments": []any{}}}})
				return openrouter.Result{Content: string(body)}, nil
			})
			_, err := d.detect(context.Background())
			var deferred *podcastContextDeferred
			if !errors.As(err, &deferred) || deferred.Left != tc.left || deferred.Right != tc.right || calls.Load() != 1 {
				t.Fatalf("missing context became a rejection or repeated request: %v, calls=%d", err, calls.Load())
			}
		})
	}
}

func TestProgressiveDetectionTrueEpisodeEdgesCanBeConfirmed(t *testing.T) {
	cues := segmentTestCues("Buy Acme storage.", "Acme protects your files.", "Visit Acme today.")
	d := progressiveTestDetector(cues, 0, 0, 0, 1)
	d.progressive.DurationMS = 30000
	var calls, callbacks atomic.Int32
	d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
		calls.Add(1)
		if !strings.Contains(input, `"context_start_is_episode_start":true`) || !strings.Contains(input, `"context_end_is_episode_end":true`) {
			return openrouter.Result{}, errors.New("known episode ends were hidden")
		}
		return progressiveTestResponse(input, cues, [2]int{0, 2})
	})
	d.confirmed = func(segments []database.PodcastSegment, _ groundedSegment) error {
		callbacks.Add(1)
		if calls.Load() != 2 || len(segments) != 1 {
			return errors.New("episode-edge proposal published before independent confirmation")
		}
		return nil
	}
	got, err := d.detect(context.Background())
	if err != nil || len(got) != 1 || got[0].StartMS != 0 || got[0].EndMS != 30000 || callbacks.Load() != 1 {
		t.Fatalf("true episode boundaries could not be finalized: %+v, %v", got, err)
	}
}

func TestProgressiveDetectionKeepsMultiMinuteAdsAndAudibleGaps(t *testing.T) {
	for _, tc := range []struct {
		name       string
		texts      []string
		bounds     [][2]int
		candidates [][2]int
		want       [][2]int
	}{
		{"four minute ad", []string{"Editorial opening.", "Buy Acme storage.", "Acme protects your files.", "Editorial returns."}, [][2]int{{600000, 610000}, {610000, 730000}, {730000, 850000}, {850000, 860000}}, [][2]int{{1, 2}}, [][2]int{{610000, 850000}}},
		{"editorial gap stays audible", []string{"Editorial opening.", "Buy Acme storage.", "An unrelated editorial discussion.", "Visit Acme today.", "Editorial returns."}, [][2]int{{600000, 610000}, {610000, 670000}, {670000, 700000}, {700000, 760000}, {760000, 770000}}, [][2]int{{1, 1}, {3, 3}}, [][2]int{{610000, 670000}, {700000, 760000}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cues := segmentTestCues(tc.texts...)
			for i, bounds := range tc.bounds {
				cues[i].StartMS, cues[i].EndMS = bounds[0], bounds[1]
			}
			d := progressiveTestDetector(cues, 1, 1, 1, 3)
			d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				return progressiveTestResponse(input, cues, tc.candidates...)
			})
			got, err := d.detect(context.Background())
			if err != nil || len(got) != len(tc.want) {
				t.Fatalf("unexpected grounded intervals: %+v, %v", got, err)
			}
			for i, want := range tc.want {
				if got[i].StartMS != want[0] || got[i].EndMS != want[1] || got[i].StartText != cues[got[i].StartCue].Text || got[i].EndText != cues[got[i].EndCue].Text {
					t.Errorf("skip lost source grounding or timing: %+v, want %v", got[i], want)
				}
			}
		})
	}
}

func TestProgressiveDetectionPublishesOnlyAfterBoundaryReview(t *testing.T) {
	for _, accept := range []bool{true, false} {
		t.Run(fmt.Sprintf("boundary accepted=%v", accept), func(t *testing.T) {
			cues := segmentTestCues("Editorial opening.", "A fictional setup.", "Acme solves your storage problems.", "A closing commercial callback.", "Editorial returns.")
			for i := range cues {
				cues[i].StartMS += podcastChunkMS
				cues[i].EndMS += podcastChunkMS
			}
			d := progressiveTestDetector(cues, 1, 1, 1, 3)
			var events []string
			d.client = segmentCompleteFunc(func(_ context.Context, prompt, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				switch {
				case strings.HasPrefix(prompt, segmentDiscoveryPrompt):
					events = append(events, "discovery")
					return progressiveTestResponse(input, cues, [2]int{2, 2})
				case strings.HasPrefix(prompt, segmentBoundaryReviewPrompt):
					events = append(events, "boundary review")
					if !accept {
						return progressiveTestResponse(input, cues)
					}
					return progressiveTestResponse(input, cues, [2]int{1, 3})
				case strings.HasPrefix(prompt, segmentConfirmationPrompt):
					events = append(events, "confirmation")
					return progressiveTestResponse(input, cues, [2]int{1, 3})
				default:
					return openrouter.Result{}, errors.New("unexpected model pass")
				}
			})
			d.confirmed = func(segments []database.PodcastSegment, _ groundedSegment) error {
				events = append(events, "publish")
				if (!accept && len(segments) != 0) || (accept && (len(segments) != 1 || segments[0].StartCue != 1 || segments[0].EndCue != 3)) {
					return errors.New("callback received unreviewed or clipped proposal")
				}
				return nil
			}
			got, err := d.detect(context.Background())
			wantCount := 0
			if accept {
				wantCount = 1
			}
			if err != nil || len(got) != wantCount || !slices.Equal(events, []string{"discovery", "confirmation", "boundary review", "publish"}) {
				t.Fatalf("publication preceded independent validation: %+v, %v, events=%v", got, err, events)
			}
		})
	}
}

func TestProgressiveDetectionLaterReviewsCannotFinalizeTemporaryEdges(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		confirmation, boundary [2]int
		left, right            bool
		calls                  int32
	}{
		{"confirmation left", [2]int{0, 2}, [2]int{}, true, false, 2},
		{"confirmation right", [2]int{2, 4}, [2]int{}, false, true, 2},
		{"boundary left", [2]int{1, 3}, [2]int{0, 3}, true, false, 3},
		{"boundary right", [2]int{1, 3}, [2]int{1, 4}, false, true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cues := segmentTestCues("Commercial setup.", "Acme storage.", "Buy Acme storage today.", "Acme storage callback.", "Commercial closing.")
			for i := range cues {
				cues[i].StartMS += podcastChunkMS
				cues[i].EndMS += podcastChunkMS
			}
			d := progressiveTestDetector(cues, 1, 1, 1, 3)
			var calls, callbacks atomic.Int32
			d.client = segmentCompleteFunc(func(_ context.Context, prompt, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				calls.Add(1)
				switch {
				case strings.HasPrefix(prompt, segmentDiscoveryPrompt):
					return progressiveTestResponse(input, cues, [2]int{2, 2})
				case strings.HasPrefix(prompt, segmentBoundaryReviewPrompt):
					return progressiveTestResponse(input, cues, tc.boundary)
				default:
					return progressiveTestResponse(input, cues, tc.confirmation)
				}
			})
			d.confirmed = func([]database.PodcastSegment, groundedSegment) error { callbacks.Add(1); return nil }
			got, err := d.detect(context.Background())
			var deferred *podcastContextDeferred
			if !errors.As(err, &deferred) || deferred.Left != tc.left || deferred.Right != tc.right || len(got) != 0 || calls.Load() != tc.calls || callbacks.Load() != 0 {
				t.Fatalf("later review finalized temporary boundary: %+v, %v, calls=%d callbacks=%d", got, err, calls.Load(), callbacks.Load())
			}
		})
	}
}

func TestProgressiveDetectionRejectedConfirmationNeverPublishes(t *testing.T) {
	cues := segmentTestCues("Editorial.", "Buy Acme storage.", "Editorial returns.")
	for i := range cues {
		cues[i].StartMS += podcastChunkMS
		cues[i].EndMS += podcastChunkMS
	}
	d := progressiveTestDetector(cues, 1, 1, 1, 3)
	var callbacks atomic.Int32
	d.client = segmentCompleteFunc(func(_ context.Context, prompt, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
		if strings.HasPrefix(prompt, segmentDiscoveryPrompt) {
			return progressiveTestResponse(input, cues, [2]int{1, 1})
		}
		return progressiveTestResponse(input, cues)
	})
	d.confirmed = func([]database.PodcastSegment, groundedSegment) error { callbacks.Add(1); return nil }
	got, err := d.detect(context.Background())
	if err != nil || len(got) != 0 || callbacks.Load() != 0 {
		t.Fatalf("rejected sponsor proposal was published: %+v, %v, callbacks=%d", got, err, callbacks.Load())
	}
}

func TestProgressiveDetectionPublishesNonSponsorsWhileDiscoveryRemainsBlocked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	texts := make([]string, 60)
	for i := range texts {
		texts[i] = fmt.Sprintf("Editorial sentence %d.", i)
	}
	texts[1], texts[2] = "Subscribe to this show.", "Buy Acme storage today."
	cues := segmentTestCues(texts...)
	d := progressiveTestDetector(cues, 0, 0, 0, 1)
	d.limiter = make(chan struct{}, 2)
	blockedDiscovery, releaseDiscovery := make(chan struct{}), make(chan struct{})
	confirmationEntered, releaseConfirmation := make(chan struct{}), make(chan struct{})
	var discoveryOnce, confirmationOnce sync.Once
	unblockDiscovery := func() { discoveryOnce.Do(func() { close(releaseDiscovery) }) }
	unblockConfirmation := func() { confirmationOnce.Do(func() { close(releaseConfirmation) }) }
	defer unblockDiscovery()
	defer unblockConfirmation()
	var discoveryComplete, confirmationComplete atomic.Bool
	published := make(chan []database.PodcastSegment, 4)
	d.client = segmentCompleteFunc(func(ctx context.Context, prompt, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
		var payload progressiveTestRequest
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return openrouter.Result{}, err
		}
		if strings.HasPrefix(prompt, segmentDiscoveryPrompt) {
			if len(payload.Windows) == 1 {
				close(blockedDiscovery)
				select {
				case <-releaseDiscovery:
				case <-ctx.Done():
					return openrouter.Result{}, ctx.Err()
				}
				discoveryComplete.Store(true)
				return progressiveTestResponseWithSegments(input)
			}
			select {
			case <-blockedDiscovery:
			case <-ctx.Done():
				return openrouter.Result{}, ctx.Err()
			}
			return progressiveTestResponseWithSegments(input, segmentTestAnswer(cues, 1, 1, "interaction"), segmentTestAnswer(cues, 2, 2, "sponsor"))
		}
		if !discoveryComplete.Load() {
			return openrouter.Result{}, errors.New("sponsor confirmation started before all discovery completed")
		}
		close(confirmationEntered)
		select {
		case <-releaseConfirmation:
		case <-ctx.Done():
			return openrouter.Result{}, ctx.Err()
		}
		confirmationComplete.Store(true)
		return progressiveTestResponse(input, cues, [2]int{2, 2})
	})
	d.confirmed = func(segments []database.PodcastSegment, span groundedSegment) error {
		if span.category == "sponsor" && !confirmationComplete.Load() {
			return errors.New("sponsor published before independent confirmation")
		}
		published <- segments
		return nil
	}
	done := make(chan error, 1)
	go func() {
		segments, err := d.detect(ctx)
		if err == nil && (len(segments) != 2 || segments[0].Category != "interaction" || segments[1].Category != "sponsor") {
			err = fmt.Errorf("final segments lost independently published results: %+v", segments)
		}
		done <- err
	}()
	select {
	case segments := <-published:
		if discoveryComplete.Load() || len(segments) != 1 || segments[0].Category != "interaction" {
			t.Fatalf("first discovery result was delayed or exposed a sponsor: %+v", segments)
		}
	case err := <-done:
		t.Fatalf("detection ended before early interaction publication: %v", err)
	case <-ctx.Done():
		t.Fatal("finished discovery batch waited for the blocked batch before publishing")
	}
	unblockDiscovery()
	select {
	case <-confirmationEntered:
	case err := <-done:
		t.Fatalf("sponsor was not independently confirmed: %v", err)
	case <-ctx.Done():
		t.Fatal("sponsor confirmation did not start after discovery completed")
	}
	select {
	case segments := <-published:
		t.Fatalf("sponsor callback fired while confirmation was blocked: %+v", segments)
	default:
	}
	unblockConfirmation()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("detection did not finish after confirmation was released")
	}
	select {
	case segments := <-published:
		if len(segments) != 1 || segments[0].Category != "sponsor" {
			t.Fatalf("confirmed sponsor publication = %+v", segments)
		}
	case <-ctx.Done():
		t.Fatal("confirmed sponsor never reached publication callback")
	}
}

func TestProgressiveDetectionInvalidPackFallbackRunsConcurrentlyWithinLimiter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	texts := make([]string, 48)
	for i := range texts {
		texts[i] = fmt.Sprintf("Editorial sentence %d.", i)
	}
	cues := segmentTestCues(texts...)
	d := progressiveTestDetector(cues, 0, 0, 0, 1)
	d.progressive.DurationMS = 480000
	d.limiter = make(chan struct{}, 2)
	var calls, active, maximum, invalid atomic.Int32
	d.sharedCalls = &calls
	entered := make(chan int, 4)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	d.client = segmentCompleteFunc(func(ctx context.Context, _ string, input string, options openrouter.CompletionOptions) (openrouter.Result, error) {
		var payload progressiveTestRequest
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return openrouter.Result{}, err
		}
		if len(payload.Windows) != 1 {
			invalid.Add(1)
			return openrouter.Result{Content: `{"windows":[]}`}, nil
		}
		if options.ReasoningEffort != "high" {
			return openrouter.Result{}, errors.New("independent fallback did not use high-effort review")
		}
		current := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
		}
		entered <- payload.Windows[0].ID
		select {
		case <-release:
			return progressiveTestResponseWithSegments(input)
		case <-ctx.Done():
			return openrouter.Result{}, ctx.Err()
		}
	})
	done := make(chan error, 1)
	go func() {
		segments, err := d.detect(ctx)
		if err == nil && len(segments) != 0 {
			err = fmt.Errorf("empty independent reviews produced skips: %+v", segments)
		}
		done <- err
	}()
	first := -1
	for range 2 {
		select {
		case id := <-entered:
			if id == first {
				t.Fatal("concurrent fallback requests repeated the same window")
			}
			first = id
		case err := <-done:
			t.Fatalf("fallback ended without concurrent independent reviews: %v", err)
		case <-ctx.Done():
			t.Fatal("independent fallback serialized requests behind a blocked window")
		}
	}
	if active.Load() != 2 {
		t.Fatalf("shared limiter admitted %d blocked fallback requests, want 2", active.Load())
	}
	unblock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("fallback did not finish after provider responses were released")
	}
	if maximum.Load() != 2 || active.Load() != 0 || invalid.Load() != 2 || calls.Load() != 6 || len(entered) != 2 {
		t.Fatalf("fallback bypassed limit, repair, or complete window coverage: max=%d active=%d invalid=%d calls=%d remaining=%d", maximum.Load(), active.Load(), invalid.Load(), calls.Load(), len(entered))
	}
}

func TestProgressiveDetectionIntroDurationGuardPreservesLongSponsors(t *testing.T) {
	for _, tc := range []struct {
		name, category string
		duration       int
		wantSkip       bool
	}{
		{"brief intro", "intro", 20000, true},
		{"one-minute intro", "intro", 60000, true},
		{"intro over one minute", "intro", 61000, false},
		{"editorial cold open mislabeled intro", "intro", 158000, false},
		{"four-minute sponsor", "sponsor", 240000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cues := segmentTestCues("Editorial opening.", "A complete opening segment.", "The main episode follows.")
			cues[0].StartMS, cues[0].EndMS = 0, 1000
			cues[1].StartMS, cues[1].EndMS = 1000, 1000+tc.duration
			cues[2].StartMS, cues[2].EndMS = 1000+tc.duration, 11000+tc.duration
			d := progressiveTestDetector(cues, 0, 0, 0, 1)
			d.progressive.DurationMS = cues[2].EndMS
			d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				return progressiveTestResponseWithSegments(input, segmentTestAnswer(cues, 1, 1, tc.category))
			})
			var published []database.PodcastSegment
			d.confirmed = func(segments []database.PodcastSegment, _ groundedSegment) error {
				published = append(published, segments...)
				return nil
			}
			segments, err := d.detect(context.Background())
			want := 0
			if tc.wantSkip {
				want = 1
			}
			if err != nil || len(segments) != want || !slices.Equal(segments, published) {
				t.Fatalf("duration guard changed skip/publication behavior: %+v, published=%+v, error=%v", segments, published, err)
			}
			if want == 1 && (segments[0].Category != tc.category || segments[0].StartMS != cues[1].StartMS || segments[0].EndMS != cues[1].EndMS) {
				t.Fatalf("retained segment lost grounded timing: %+v", segments[0])
			}
		})
	}
}

func TestProgressiveDetectionIntroDurationGuardAppliesAfterAdjacentSpansMerge(t *testing.T) {
	for _, tc := range []struct {
		name, category string
		cueDuration    int
		wantSkip       bool
	}{
		{"adjacent brief intros exceed one minute together", "intro", 40000, false},
		{"adjacent intros total one minute", "intro", 30000, true},
		{"adjacent sponsor spans total four minutes", "sponsor", 120000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cues := segmentTestCues("Editorial opening.", "A complete opening segment.", "The opening segment continues.", "The main episode follows.")
			cues[0].StartMS, cues[0].EndMS = 0, 1000
			cues[1].StartMS, cues[1].EndMS = 1000, 1000+tc.cueDuration
			cues[2].StartMS, cues[2].EndMS = cues[1].EndMS, 1000+2*tc.cueDuration
			cues[3].StartMS, cues[3].EndMS = cues[2].EndMS, cues[2].EndMS+10000
			d := progressiveTestDetector(cues, 0, 0, 0, 1)
			d.progressive.DurationMS = cues[3].EndMS
			d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				return progressiveTestResponseWithSegments(input, segmentTestAnswer(cues, 1, 1, tc.category), segmentTestAnswer(cues, 2, 2, tc.category))
			})
			segments, err := d.detect(context.Background())
			want := 0
			if tc.wantSkip {
				want = 1
			}
			if err != nil || len(segments) != want {
				t.Fatalf("merged duration guard produced unsafe intervals: %+v, error=%v", segments, err)
			}
			if want == 1 && (segments[0].StartMS != cues[1].StartMS || segments[0].EndMS != cues[2].EndMS || segments[0].StartText != cues[1].Text || segments[0].EndText != cues[2].Text) {
				t.Fatalf("retained merged segment lost cue grounding: %+v", segments[0])
			}
		})
	}
}

func TestProgressiveDetectionSharesEpisodeRequestBudget(t *testing.T) {
	var budget, requests atomic.Int32
	budget.Store(segmentMaxCalls - 1)
	for i := range 2 {
		cues := segmentTestCues("Editorial.")
		d := progressiveTestDetector(cues, 0, 0, 0, 1)
		d.sharedCalls = &budget
		d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
			requests.Add(1)
			return progressiveTestResponse(input, cues)
		})
		_, err := d.review(context.Background(), []segmentWindow{d.window(0, 0, 0)}, segmentDiscoveryPrompt, "medium", false, 0)
		if (i == 0 && err != nil) || (i == 1 && (err == nil || !strings.Contains(err.Error(), "request limit"))) {
			t.Fatalf("core %d did not use episode budget: %v", i, err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("new detector bypassed episode budget: %d provider requests", requests.Load())
	}
}

func TestProgressiveDetectionSharesProviderConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	limiter := make(chan struct{}, 2)
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	var closeOnce sync.Once
	unblock := func() { closeOnce.Do(func() { close(release) }) }
	defer unblock()
	var active, maximum, requests, budget atomic.Int32
	results := make(chan error, 8)
	for range 8 {
		go func() {
			cues := segmentTestCues("Editorial.")
			d := progressiveTestDetector(cues, 0, 0, 0, 1)
			d.sharedCalls, d.limiter = &budget, limiter
			d.client = segmentCompleteFunc(func(ctx context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				requests.Add(1)
				current := active.Add(1)
				defer active.Add(-1)
				for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
				}
				entered <- struct{}{}
				select {
				case <-release:
					return progressiveTestResponse(input, cues)
				case <-ctx.Done():
					return openrouter.Result{}, ctx.Err()
				}
			})
			_, err := d.review(ctx, []segmentWindow{d.window(0, 0, 0)}, segmentDiscoveryPrompt, "medium", false, 0)
			results <- err
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("provider limit never admitted two requests")
		}
	}
	select {
	case <-entered:
		t.Error("third core entered provider while two requests were held")
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	for range 8 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if maximum.Load() != 2 || active.Load() != 0 || requests.Load() != 8 || budget.Load() != 8 {
		t.Fatalf("episode concurrency accounting failed: max=%d active=%d requests=%d budget=%d", maximum.Load(), active.Load(), requests.Load(), budget.Load())
	}
}
