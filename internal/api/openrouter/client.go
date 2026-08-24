// Package openrouter contains the narrow, transcript-segment-only provider client.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultModel = "google/gemini-3.5-flash-lite"

type Client struct {
	key, model string
	http       *http.Client
}
type CompletionOptions struct {
	MaxTokens       int
	ReasoningEffort string
}
type Usage struct {
	Model                     string
	InputTokens, OutputTokens int
	Cost                      float64
}
type Result struct {
	Content string
	Usage   Usage
}

var DefaultClient *Client

func init() {
	DefaultClient = NewFromEnvironment()
}

// NewFromEnvironment constructs the narrowly configured client after callers
// load deployment configuration (useful for local tools and tests).
func NewFromEnvironment() *Client {
	key := strings.TrimSpace(os.Getenv("PODCAST_SEGMENTS_OPENROUTER_API_KEY"))
	if key == "" || strings.EqualFold(strings.TrimSpace(os.Getenv("PODCAST_SEGMENTS_ENABLED")), "false") {
		return nil
	}
	model := strings.TrimSpace(os.Getenv("PODCAST_SEGMENTS_MODEL"))
	if model == "" {
		model = defaultModel
	}
	return New(key, model)
}

func New(key, model string) *Client {
	return &Client{key: strings.TrimSpace(key), model: strings.TrimSpace(model), http: &http.Client{Timeout: 60 * time.Second}}
}
func (c *Client) Model() string { return c.model }

func (c *Client) Complete(ctx context.Context, system, input string) (Result, error) {
	return c.CompleteWithOptions(ctx, system, input, CompletionOptions{MaxTokens: 6000})
}

func (c *Client) CompleteWithOptions(ctx context.Context, system, input string, options CompletionOptions) (Result, error) {
	if options.MaxTokens <= 0 {
		options.MaxTokens = 6000
	}
	payload := map[string]any{"model": c.model, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": input}}, "max_tokens": options.MaxTokens, "response_format": map[string]any{"type": "json_object"}}
	if options.ReasoningEffort != "" {
		payload["reasoning"] = map[string]any{"effort": options.ReasoningEffort}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Result{}, errors.New("provider returned non-success status")
	}
	var decoded struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			Cost             float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&decoded); err != nil {
		return Result{}, err
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return Result{}, errors.New("provider returned empty response")
	}
	return Result{Content: decoded.Choices[0].Message.Content, Usage: Usage{Model: decoded.Model, InputTokens: decoded.Usage.PromptTokens, OutputTokens: decoded.Usage.CompletionTokens, Cost: decoded.Usage.Cost}}, nil
}
