package types

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
)

func TestPlayerSegmentsPreservePrecisionAndCategory(t *testing.T) {
	var props VideoPlayerProps
	err := props.AddSegments(
		sponsorblock.Segment{Segment: []float64{20.5, 40.25}, Category: "sponsor"},
		sponsorblock.Segment{Segment: []float64{41.75, 50.1}, Category: "selfpromo"},
		sponsorblock.Segment{Segment: []float64{math.NaN(), 20}},
		sponsorblock.Segment{Segment: []float64{0, math.Inf(1)}},
		sponsorblock.Segment{Segment: []float64{-1, 20}},
		sponsorblock.Segment{Segment: []float64{20, 10}},
		sponsorblock.Segment{Segment: []float64{20, 20}},
		sponsorblock.Segment{Segment: []float64{20}},
	)
	if err != nil {
		t.Fatal(err)
	}
	var segments []SegmentProps
	if err := json.Unmarshal([]byte(props.SkipSegmentsJSON), &segments); err != nil {
		t.Fatal(err)
	}
	if len(segments) != 2 || segments[0].Start != 20.5 || segments[0].End != 40.25 || segments[0].Category != "sponsor" || segments[1].Category != "selfpromo" {
		t.Fatalf("precision or category lost: %+v", segments)
	}
}
