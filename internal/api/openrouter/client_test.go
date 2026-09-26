package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestModelEnvironmentOverride(t *testing.T) {
	t.Setenv("PODCAST_SEGMENTS_OPENROUTER_API_KEY", "test-key")
	t.Setenv("PODCAST_SEGMENTS_ENABLED", "true")
	t.Setenv("PODCAST_SEGMENTS_MODEL", " custom/model ")
	if got := NewFromEnvironment().Model(); got != "custom/model" {
		t.Fatalf("environment model ignored: %q", got)
	}
	t.Setenv("PODCAST_SEGMENTS_MODEL", "")
	if got := NewFromEnvironment().Model(); got != DefaultModel {
		t.Fatalf("unexpected fallback: %q", got)
	}
	t.Setenv("PODCAST_SEGMENTS_ENABLED", "false")
	if NewFromEnvironment() != nil {
		t.Fatal("disabled client should be nil")
	}
}

func TestReasoningAndIncompleteResponses(t *testing.T) {
	for _, finish := range []string{"stop", "length", "content_filter", ""} {
		t.Run(finish, func(t *testing.T) {
			c := New("test-key", "configured/model")
			c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				var request struct {
					Model     string `json:"model"`
					MaxTokens int    `json:"max_tokens"`
					Reasoning struct {
						Effort  string `json:"effort"`
						Exclude bool   `json:"exclude"`
					} `json:"reasoning"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request.Model != "configured/model" || request.MaxTokens != 16384 || request.Reasoning.Effort != "high" || !request.Reasoning.Exclude {
					t.Fatalf("wrong request options: %+v", request)
				}
				body, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]string{"content": `{"windows":[]}`}}}})
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			})}
			_, err := c.CompleteWithOptions(context.Background(), "system", "input", CompletionOptions{MaxTokens: 16384, ReasoningEffort: "high"})
			if (err == nil) != (finish == "stop") {
				t.Fatalf("finish=%q, error=%v", finish, err)
			}
		})
	}
}
