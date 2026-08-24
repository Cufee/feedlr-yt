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

func TestSanitizePodcastHTMLLinkifiesPlainURLs(t *testing.T) {
	input := `<p>Visit https://example.com/docs and www.feedlr.app.</p><a href="https://existing.example">Existing link</a><pre>https://code.example</pre>`
	got := SanitizePodcastHTML(input)

	for _, href := range []string{"https://example.com/docs", "https://www.feedlr.app", "https://existing.example"} {
		if !strings.Contains(got, `href="`+href+`"`) {
			t.Fatalf("missing linked URL %q: %q", href, got)
		}
	}
	if strings.Count(got, `href="https://existing.example"`) != 1 {
		t.Fatalf("existing link was rewritten: %q", got)
	}
	if strings.Contains(got, `href="https://code.example"`) {
		t.Fatalf("code block URL was linkified: %q", got)
	}
	for _, attribute := range []string{`target="_blank"`, "nofollow", "noreferrer"} {
		if !strings.Contains(got, attribute) {
			t.Fatalf("linked URL is missing %q: %q", attribute, got)
		}
	}
}
