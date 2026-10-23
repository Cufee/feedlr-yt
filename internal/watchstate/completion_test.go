package watchstate

import "testing"

func TestCompletionPolicy(t *testing.T) {
	for _, test := range []struct {
		progress, duration int
		want               bool
	}{{0, 20, false}, {1, 0, false}, {0, 3600, false}, {3299, 3600, false}, {3300, 3600, true}, {100, 900, false}, {870, 900, true}, {1000, 1000, true}} {
		if got := IsCompleted(test.progress, test.duration); got != test.want {
			t.Fatalf("progress=%d duration=%d: %v want %v", test.progress, test.duration, got, test.want)
		}
	}
}
