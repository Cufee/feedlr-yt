package webapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLegalDocumentUsesAllowlistBoundsAndSanitizedContent(t *testing.T) {
	f := newFixture(t)
	previous := legalHTTPClient
	t.Cleanup(func() { legalHTTPClient = previous })
	var requested string
	response := `<h1>Privacy</h1><p>Actual legal text</p><script>alert(1)</script><a href="javascript:alert(1)">bad</a>`
	status := 200
	legalHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		requested = r.URL.String()
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(response)), Header: http.Header{"Content-Type": []string{"text/html"}}}, nil
	})}
	res, v := f.request(t, "GET", "/legal/privacy-policy", "", "", "")
	if res.StatusCode != 200 {
		t.Fatalf("legal: %d %#v", res.StatusCode, v)
	}
	f.check(t, v, "LegalDocument")
	html := v.(map[string]any)["html"].(string)
	if requested != "https://byvko-dev.github.io/legal/privacy-policy" || !strings.Contains(html, "Actual legal text") || strings.Contains(html, "<script") || strings.Contains(html, "javascript:") {
		t.Fatalf("unsafe or replaced legal document: %s (%s)", html, requested)
	}
	res, v = f.request(t, "GET", "/legal/arbitrary", "", "", "")
	if res.StatusCode != 404 {
		t.Fatalf("nonallowlisted document: %d", res.StatusCode)
	}
	f.check(t, v, "Error")
	status = 503
	res, v = f.request(t, "GET", "/legal/terms-of-service", "", "", "")
	if res.StatusCode != 502 {
		t.Fatalf("upstream failure: %d", res.StatusCode)
	}
	f.check(t, v, "Error")
	if requested != "https://byvko-dev.github.io/legal/terms-of-service-partial" {
		t.Fatalf("wrong terms URL: %s", requested)
	}
	status = 200
	response = strings.Repeat("x", (1<<20)+1)
	res, v = f.request(t, "GET", "/legal/privacy-policy", "", "", "")
	if res.StatusCode != 502 {
		t.Fatalf("size limit: %d", res.StatusCode)
	}
	f.check(t, v, "Error")
	legalHTTPClient = &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	res, v = f.request(t, "GET", "/legal/privacy-policy", "", "", "")
	if res.StatusCode != 502 {
		t.Fatalf("network failure: %d", res.StatusCode)
	}
	f.check(t, v, "Error")
}
