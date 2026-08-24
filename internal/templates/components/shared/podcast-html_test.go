package shared

import (
	"strings"
	"testing"
)

func TestSanitizePodcastHTML(t *testing.T) {
	input := `&lt;p&gt;Listen to &lt;strong&gt;this episode&lt;/strong&gt;.&lt;/p&gt;&lt;a href="javascript:alert(1)" onclick="alert(1)"&gt;bad link&lt;/a&gt;&lt;script&gt;alert(1)&lt;/script&gt;`
	got := SanitizePodcastHTML(input)

	if !strings.Contains(got, "<p>Listen to <strong>this episode</strong>.</p>") {
		t.Fatalf("sanitized HTML lost safe markup: %q", got)
	}
	for _, unsafe := range []string{"javascript:", "onclick", "<script"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("sanitized HTML contains %q: %q", unsafe, got)
		}
	}
}
