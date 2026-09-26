package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
)

type segmentCompleteFunc func(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error)

func (f segmentCompleteFunc) CompleteWithOptions(ctx context.Context, prompt, input string, options openrouter.CompletionOptions) (openrouter.Result, error) {
	return f(ctx, prompt, input, options)
}

func segmentTestCues(texts ...string) []transcriptCue {
	cues := make([]transcriptCue, len(texts))
	for i, text := range texts {
		cues[i] = transcriptCue{Index: i, StartMS: i * 10_000, EndMS: (i + 1) * 10_000, Text: text}
	}
	return cues
}

func segmentTestAnswer(cues []transcriptCue, a, b int, category string) map[string]any {
	return map[string]any{"category": category, "start_cue": a, "end_cue": b, "start_quote": cues[a].Text, "end_quote": cues[b].Text, "brand": "Acme", "reason": "Dedicated product commercial", "evidence": []any{map[string]any{"cue_id": a, "quote": cues[a].Text}}}
}

func segmentTestResponse(id int, segments ...map[string]any) openrouter.Result {
	if segments == nil {
		segments = []map[string]any{}
	}
	body, _ := json.Marshal(map[string]any{"windows": []any{map[string]any{"window_id": id, "reviewed": true, "need_left": false, "need_right": false, "segments": segments}}})
	return openrouter.Result{Content: string(body)}
}

func TestSegmentGroundingRejectsWrongCues(t *testing.T) {
	cues := segmentTestCues("Today we discuss storage.", "Editorial. Buy Acme storage today.", "Visit Acme to sign up. Back to the interview.")
	d := segmentDetector{cues: cues}
	valid := segmentTestAnswer(cues, 1, 2, "sponsor")
	valid["start_quote"] = "Buy Acme storage today."
	valid["end_quote"] = "Visit Acme to sign up."
	valid["evidence"] = []any{map[string]any{"cue_id": 1, "quote": "Buy Acme storage today."}}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"valid", func(map[string]any) {}},
		{"wrong cue", func(s map[string]any) { s["start_cue"] = 0 }},
		{"missing cue", func(s map[string]any) { delete(s, "start_cue") }},
		{"out of context", func(s map[string]any) { s["end_cue"] = 3 }},
		{"outside core", func(s map[string]any) { s["start_cue"] = 0; s["end_cue"] = 0 }},
		{"evidence outside span", func(s map[string]any) { s["evidence"] = []any{map[string]any{"cue_id": 1, "quote": "Editorial."}} }},
		{"invented quote", func(s map[string]any) { s["end_quote"] = "A made up ending." }},
		{"missing evidence", func(s map[string]any) { delete(s, "evidence") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := map[string]any{}
			for k, v := range valid {
				copy[k] = v
			}
			tc.mutate(copy)
			raw, _ := json.Marshal(copy)
			var answer segmentAnswer
			_ = json.Unmarshal(raw, &answer)
			_, err := d.ground(answer, segmentWindow{coreStart: 1, coreEnd: 2, from: 0, to: 2}, true)
			if (err == nil) != (tc.name == "valid") {
				t.Fatalf("unexpected grounding result: %v", err)
			}
		})
	}
	if _, err := quoteOffset("Acme Acme", "Acme"); err == nil {
		t.Fatal("ambiguous quote accepted")
	}
}

func TestSupportedInteriorsKeepMixedCuesAndGapsAudible(t *testing.T) {
	cues := segmentTestCues("Editorial. An ad begins.", "First commercial sentence.", "Unrelated editorial discussion.", "Second commercial sentence.", "The ad ends. Editorial resumes.")
	d := segmentDetector{cues: cues}
	final := groundedSegment{start: segmentPoint{0, len("Editorial. ")}, end: segmentPoint{4, len("The ad ends.")}, category: "sponsor", brand: "Acme"}
	first, last := final, final
	first.end = segmentPoint{1, len(cues[1].Text)}
	last.start = segmentPoint{3, 0}
	got := d.supportedInteriors(final, []groundedSegment{first, last})
	if len(got) != 2 || got[0].StartCue != 1 || got[0].EndCue != 1 || got[1].StartCue != 3 || got[1].EndCue != 3 {
		t.Fatalf("mixed boundaries or gap consumed: %+v", got)
	}
	// A narrower overlapping discovery must not erase the larger supported body.
	narrow := first
	narrow.start = segmentPoint{1, 0}
	got = d.supportedInteriors(final, []groundedSegment{final, narrow})
	if len(got) != 1 || got[0].StartCue != 1 || got[0].EndCue != 3 {
		t.Fatalf("support union clipped ad: %+v", got)
	}
	if got = d.supportedInteriors(final, nil); len(got) != 0 {
		t.Fatal("unsupported confirmation authorized a skip")
	}
}

func TestIndependentSponsorConfirmationCanReject(t *testing.T) {
	cues := segmentTestCues("Try our free project.", "Please subscribe to the show.")
	calls := 0
	d := segmentDetector{cues: cues, title: "A guest project", notes: `["Acme"]`}
	d.client = segmentCompleteFunc(func(_ context.Context, prompt, input string, options openrouter.CompletionOptions) (openrouter.Result, error) {
		calls++
		if calls == 1 {
			if options.ReasoningEffort != "medium" {
				t.Fatal("discovery effort")
			}
			return segmentTestResponse(0, segmentTestAnswer(cues, 0, 0, "sponsor"), segmentTestAnswer(cues, 1, 1, "interaction")), nil
		}
		if !strings.HasPrefix(prompt, segmentConfirmationPrompt) || options.ReasoningEffort != "high" {
			t.Fatal("missing independent confirmation")
		}
		if strings.Contains(input, "Acme") || strings.Contains(input, "candidate") {
			t.Fatal("prior brand or verdict supplied as truth")
		}
		return segmentTestResponse(0), nil
	})
	got, err := d.detect(context.Background())
	if err != nil || calls != 2 || len(got) != 1 || got[0].Category != "interaction" {
		t.Fatalf("unconfirmed ad leaked or category lost: %+v, %v, %d calls", got, err, calls)
	}
}

func TestSelectiveBoundaryReview(t *testing.T) {
	cues := segmentTestCues("The interview starts.", "Imagine a fictional office.", "Acme makes storage simple.", "And the fictional team rejoiced.", "Now our actual guest returns.")
	for _, mode := range []string{"recover", "reject", "fail", "widen", "already supported"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			d := segmentDetector{cues: cues}
			d.client = segmentCompleteFunc(func(_ context.Context, prompt, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
				calls++
				switch calls {
				case 1:
					if mode == "already supported" {
						return segmentTestResponse(0, segmentTestAnswer(cues, 1, 3, "sponsor")), nil
					}
					return segmentTestResponse(0, segmentTestAnswer(cues, 2, 2, "sponsor")), nil
				case 2:
					id := 2
					if mode == "already supported" {
						id = 1
					}
					return segmentTestResponse(id, segmentTestAnswer(cues, 1, 3, "sponsor")), nil
				case 3:
					if !strings.HasPrefix(prompt, segmentBoundaryReviewPrompt) {
						t.Fatal("expected focused review")
					}
					if mode == "fail" {
						return openrouter.Result{}, errors.New("provider unavailable")
					}
					if mode == "reject" {
						return segmentTestResponse(1), nil
					}
					if mode == "widen" {
						return segmentTestResponse(1, segmentTestAnswer(cues, 0, 4, "sponsor")), nil
					}
					return segmentTestResponse(1, segmentTestAnswer(cues, 1, 3, "sponsor")), nil
				}
				t.Fatal("unnecessary model call")
				return openrouter.Result{}, errors.New("extra call")
			})
			got, err := d.detect(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if mode == "reject" {
				if len(got) != 0 {
					t.Fatal("rejected ad skipped")
				}
				return
			}
			start, end := 1, 3
			if mode == "fail" {
				start, end = 2, 2
			}
			if len(got) != 1 || got[0].StartCue != start || got[0].EndCue != end {
				t.Fatalf("wrong safe skip: %+v", got)
			}
			if mode == "already supported" && calls != 2 {
				t.Fatalf("unnecessary boundary calls: %d", calls)
			}
		})
	}
}

func TestWindowDecisionsMustBeComplete(t *testing.T) {
	d := segmentDetector{cues: segmentTestCues("No commercial here.")}
	for _, content := range []string{`{}`, `{"windows":[]}`, `{"windows":[{"window_id":0,"reviewed":true,"need_left":false,"need_right":false}]}`, `{"windows":[{"window_id":99,"reviewed":true,"need_left":false,"need_right":false,"segments":[]}]}`, `{"windows":[{"window_id":0,"reviewed":true,"need_left":false,"need_right":false,"segments":[]},{"window_id":0,"reviewed":true,"need_left":false,"need_right":false,"segments":[]}]}`} {
		if _, err := d.decode(content, []segmentWindow{d.window(0, 0, 0)}, false); err == nil {
			t.Fatalf("accepted incomplete result: %s", content)
		}
	}
	calls := 0
	d.client = segmentCompleteFunc(func(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error) {
		calls++
		return openrouter.Result{Content: `{}`}, nil
	})
	if _, err := d.detect(context.Background()); err == nil || calls != 4 {
		t.Fatalf("failure silently became ad-free, or retry unbounded: err=%v calls=%d", err, calls)
	}
}

func TestContextExpansionAndRepair(t *testing.T) {
	cues := segmentTestCues("An editorial opening.", "A fictional setup.", "Acme solves your storage problems.", "The closing callback.", "Editorial again.")
	for i := range cues {
		cues[i].StartMS = i * 60_000
		cues[i].EndMS = (i + 1) * 60_000
	}
	d := segmentDetector{cues: cues}
	calls := 0
	d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
		calls++
		if calls == 1 {
			return openrouter.Result{Content: `{"windows":[{"window_id":2,"reviewed":true,"need_left":true,"need_right":true,"segments":[]}]}`}, nil
		}
		if !strings.Contains(input, `"core":[2,2]`) || !strings.Contains(input, "fictional setup") || !strings.Contains(input, "closing callback") {
			t.Fatal("expansion lost core or source context")
		}
		if calls == 2 {
			bad := segmentTestAnswer(cues, 1, 3, "sponsor")
			bad["start_quote"] = "Invented words"
			return segmentTestResponse(2, bad), nil
		}
		if !strings.Contains(input, "validation_feedback") || !strings.Contains(input, "invalid_answer") {
			t.Fatal("repair lacks invalid answer")
		}
		return segmentTestResponse(2, segmentTestAnswer(cues, 1, 3, "sponsor")), nil
	})
	got, err := d.review(context.Background(), []segmentWindow{d.window(2, 2, 0)}, segmentDiscoveryPrompt, "medium", false, 0)
	if err != nil || calls != 3 || len(got) != 1 || got[0].start.cue != 1 || got[0].end.cue != 3 {
		t.Fatalf("expansion failed: %+v %v", got, err)
	}
}

func TestContextExpansionAndRequestLimits(t *testing.T) {
	cues := segmentTestCues("Editorial.")
	for _, mode := range []string{"unavailable context", "expansion cap", "request cap", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			d := segmentDetector{cues: cues}
			calls := 0
			d.client = segmentCompleteFunc(func(context.Context, string, string, openrouter.CompletionOptions) (openrouter.Result, error) {
				calls++
				return openrouter.Result{Content: `{"windows":[{"window_id":0,"reviewed":true,"need_left":true,"need_right":true,"segments":[]}]}`}, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			round := 0
			if mode == "expansion cap" {
				round = segmentExpansionRounds
			}
			if mode == "request cap" {
				d.calls.Store(segmentMaxCalls)
			}
			if mode == "cancelled" {
				cancel()
			}
			if _, err := d.review(ctx, []segmentWindow{d.window(0, 0, 0)}, segmentDiscoveryPrompt, "medium", false, round); err == nil {
				t.Fatal("unresolved response accepted")
			}
			if (mode == "request cap" || mode == "cancelled") && calls != 0 {
				t.Fatal("request made past limit")
			}
		})
	}
}

func TestWindowCoverageAndPacking(t *testing.T) {
	var texts []string
	for i := 0; i < 150; i++ {
		texts = append(texts, fmt.Sprintf("Editorial sentence %d.", i))
	}
	d := segmentDetector{cues: segmentTestCues(texts...)}
	covered := make([]bool, len(texts))
	for _, w := range d.windows() {
		for i := w.coreStart; i <= w.coreEnd; i++ {
			if covered[i] {
				t.Fatal("duplicate core")
			}
			covered[i] = true
		}
	}
	for i, ok := range covered {
		if !ok {
			t.Fatalf("cue %d never scanned", i)
		}
	}
	d.client = segmentCompleteFunc(func(_ context.Context, _ string, input string, _ openrouter.CompletionOptions) (openrouter.Result, error) {
		var payload struct {
			Windows []struct {
				ID int `json:"window_id"`
			} `json:"windows"`
		}
		if err := json.Unmarshal([]byte(input), &payload); err != nil {
			return openrouter.Result{}, err
		}
		if len(payload.Windows) > segmentPack {
			return openrouter.Result{}, errors.New("oversized pack")
		}
		var answers []any
		for _, w := range payload.Windows {
			answers = append(answers, map[string]any{"window_id": w.ID, "reviewed": true, "need_left": false, "need_right": false, "segments": []any{}})
		}
		body, _ := json.Marshal(map[string]any{"windows": answers})
		return openrouter.Result{Content: string(body)}, nil
	})
	got, err := d.detect(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("negative corpus: %+v %v", got, err)
	}
	want := (len(d.windows()) + segmentPack - 1) / segmentPack
	if int(d.calls.Load()) != want {
		t.Fatalf("expected %d packed requests, got %d", want, d.calls.Load())
	}
}

func TestMergeSegmentsPreservesEndpointsAndGaps(t *testing.T) {
	got := mergePodcastSegments([]database.PodcastSegment{
		{Category: "sponsor", Brand: "Acme", StartMS: 0, EndMS: 20, StartCue: 0, EndCue: 1, EndText: "first end"},
		{Category: "interaction", StartMS: 5, EndMS: 7},
		{Category: "sponsor", Brand: "Acme", StartMS: 10, EndMS: 30, StartCue: 1, EndCue: 2, EndText: "last end"},
		{Category: "sponsor", Brand: "Acme", StartMS: 40, EndMS: 50, StartCue: 4, EndCue: 5},
	})
	if len(got) != 3 || got[0].EndMS != 30 || got[0].EndCue != 2 || got[0].EndText != "last end" || got[2].StartMS != 40 {
		t.Fatalf("invalid merge: %+v", got)
	}
}
