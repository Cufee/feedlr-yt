package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const validTranscription = `{"text":"Hello there.","duration":2.5,"segments":[{"start":0,"end":1.25,"text":"Hello there."}],"usage":{"seconds":2.5,"cost":0.0002,"provider_value":"preserved"}}`

func transcriptionFile(t *testing.T) (string, []byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audio.flac")
	audio := []byte("fLaC\x00\x01test-audio")
	if err := os.WriteFile(path, audio, 0600); err != nil {
		t.Fatal(err)
	}
	return path, audio
}

func providerResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestTranscribeMultipartAndFixedModel(t *testing.T) {
	path, audio := transcriptionFile(t)
	c := New("test-key", "configured/sponsor-model")
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.String() != "https://openrouter.ai/api/v1/audio/transcriptions" {
			t.Fatalf("wrong transcription endpoint: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing configured authorization")
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("wrong multipart content type: %s, %v", mediaType, err)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		fields := make(map[string]string)
		fileCount := 0
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if part.FormName() == "file" {
				fileCount++
				if part.Header.Get("Content-Disposition") != `form-data; name="file"; filename="audio.flac"` {
					t.Fatalf("OpenRouter requires quoted multipart disposition: %q", part.Header.Get("Content-Disposition"))
				}
				if part.FileName() != "audio.flac" || part.Header.Get("Content-Type") != "audio/flac" || !bytes.Equal(data, audio) {
					t.Fatalf("wrong uploaded FLAC: name=%q type=%q data=%q", part.FileName(), part.Header.Get("Content-Type"), data)
				}
			} else {
				fields[part.FormName()] = string(data)
			}
		}
		if fileCount != 1 || len(fields) != 3 || fields["model"] != "openai/whisper-large-v3-turbo" || fields["response_format"] != "verbose_json" || fields["timestamp_granularities[]"] != "segment" {
			t.Fatalf("wrong transcription fields: %+v, files=%d", fields, fileCount)
		}
		resp := providerResponse(http.StatusOK, validTranscription)
		resp.Header.Set("X-Generation-Id", "generation-test")
		return resp, nil
	})}
	result, err := c.Transcribe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello there." || result.Duration != 2.5 || len(result.Segments) != 1 || result.Segments[0].Start != 0 || result.Segments[0].End != 1.25 || result.GenerationID != "generation-test" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if string(result.Usage) != `{"seconds":2.5,"cost":0.0002,"provider_value":"preserved"}` {
		t.Fatalf("usage was changed: %s", result.Usage)
	}
}

func TestTranscribeWithLanguageMultipart(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, requested := range []string{"en", " EN "} {
		t.Run(requested, func(t *testing.T) {
			c := New("test-key", "configured/sponsor-model")
			c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Fatal(err)
				}
				defer r.MultipartForm.RemoveAll()
				if files := r.MultipartForm.File["file"]; len(files) != 1 || files[0].Header.Get("Content-Disposition") != `form-data; name="file"; filename="audio.flac"` {
					t.Fatal("language request must use canonical quoted file disposition")
				}
				if r.FormValue("language") != "en" || r.FormValue("model") != TranscriptionModel || r.FormValue("response_format") != "verbose_json" || r.FormValue("timestamp_granularities[]") != "segment" {
					t.Fatalf("wrong multipart language/options: %+v", r.MultipartForm.Value)
				}
				return providerResponse(http.StatusOK, validTranscription), nil
			})}
			if _, err := c.TranscribeWithLanguage(context.Background(), path, requested); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTranscribePreservesReportedLanguage(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, language := range []string{"en", "english", "Russian", ""} {
		t.Run(language, func(t *testing.T) {
			body := map[string]any{"text": "", "segments": []any{}}
			if language != "" {
				body["language"] = language
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			c := New("test-key", "model")
			c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return providerResponse(http.StatusOK, string(encoded)), nil
			})}
			result, err := c.Transcribe(context.Background(), path)
			if err != nil || result.Language != language {
				t.Fatalf("reported language changed: got=%q want=%q error=%v", result.Language, language, err)
			}
		})
	}
}

func TestTranscribeWithLanguageRejectsInvalidCodes(t *testing.T) {
	c := New("test-key", "model")
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid language reached provider")
		return nil, errors.New("unexpected provider call")
	})}
	for _, language := range []string{"english", "eng", "en-US", "e", "e1", "é", "İ", "Ko"} {
		if _, err := c.TranscribeWithLanguage(context.Background(), "nonexistent.flac", language); err == nil || !strings.Contains(err.Error(), "ISO 639-1") {
			t.Fatalf("expected language validation error for %q, got %v", language, err)
		}
	}
}

func TestTranscribeSilenceAndMalformedResponses(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, tc := range []struct {
		name string
		body string
		ok   bool
	}{
		{"silence", `{"text":"","duration":2,"segments":[]}`, true},
		{"provider silence interval", `{"text":"","language":"en","duration":6,"segments":[{"start":0,"end":6,"text":""}]}`, true},
		{"invalid provider silence interval", `{"text":"","segments":[{"start":0,"end":0,"text":""}]}`, false},
		{"whitespace silence", `{"text":" \n","segments":[]}`, true},
		{"optional duration", `{"text":"Hello","segments":[{"start":0,"end":1,"text":"Hello"}]}`, true},
		{"adjacent segments", `{"text":"Hello world","segments":[{"start":0,"end":1,"text":"Hello"},{"start":1,"end":2,"text":"world"}]}`, true},
		{"missing segments", `{"text":""}`, false},
		{"null segments", `{"text":"","segments":null}`, false},
		{"missing text", `{"segments":[]}`, false},
		{"null text", `{"text":null,"segments":[]}`, false},
		{"untimestamped text", `{"text":"Hello","segments":[]}`, false},
		{"null duration", `{"text":"","duration":null,"segments":[]}`, false},
		{"negative duration", `{"text":"","duration":-1,"segments":[]}`, false},
		{"nonfinite duration", `{"text":"","duration":1e999,"segments":[]}`, false},
		{"wrong duration type", `{"text":"","duration":"2","segments":[]}`, false},
		{"missing start", `{"text":"Hello","segments":[{"end":1,"text":"Hello"}]}`, false},
		{"missing end", `{"text":"Hello","segments":[{"start":0,"text":"Hello"}]}`, false},
		{"missing segment text", `{"text":"Hello","segments":[{"start":0,"end":1}]}`, false},
		{"negative timestamp", `{"text":"Hello","segments":[{"start":-1,"end":1,"text":"Hello"}]}`, false},
		{"empty timestamp interval", `{"text":"Hello","segments":[{"start":1,"end":1,"text":"Hello"}]}`, false},
		{"reversed timestamp interval", `{"text":"Hello","segments":[{"start":2,"end":1,"text":"Hello"}]}`, false},
		{"overlapping timestamps", `{"text":"Hello world","segments":[{"start":0,"end":2,"text":"Hello"},{"start":1,"end":3,"text":"world"}]}`, false},
		{"unordered timestamps", `{"text":"Hello world","segments":[{"start":2,"end":3,"text":"Hello"},{"start":0,"end":1,"text":"world"}]}`, false},
		{"nonfinite timestamp", `{"text":"Hello","segments":[{"start":0,"end":1e999,"text":"Hello"}]}`, false},
		{"empty segment text", `{"text":"Hello","segments":[{"start":0,"end":1,"text":""}]}`, false},
		{"segments without text", `{"text":"","segments":[{"start":0,"end":1,"text":"Hello"}]}`, false},
		{"invalid JSON", `{`, false},
		{"trailing JSON", `{"text":"","segments":[]} {}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := New("test-key", "configured/model")
			c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return providerResponse(http.StatusOK, tc.body), nil
			})}
			result, err := c.Transcribe(context.Background(), path)
			if (err == nil) != tc.ok {
				t.Fatalf("error=%v, expected valid=%t", err, tc.ok)
			}
			if calls != 1 {
				t.Fatalf("decoded responses should not retry, calls=%d", calls)
			}
			if tc.ok && result.Segments == nil {
				t.Fatal("explicit segment array should remain an array")
			}
		})
	}
}

func TestTranscribeProviderErrorDiagnostics(t *testing.T) {
	path, _ := transcriptionFile(t)
	calls := 0
	c := New("fake-provider-secret", "model")
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return providerResponse(http.StatusBadRequest, `{"error":{"message":"Provider returned error: fake-provider-secret","metadata":{"provider_name":"Groq","raw":"{\"error\":{\"message\":\"Invalid language en\\n fake-provider-secret\",\"sensitive_extra\":\"must not expose\"}}"}}}`), nil
	})}
	_, err := c.TranscribeWithLanguage(context.Background(), path, "en")
	var providerError *TranscriptionProviderError
	if !errors.As(err, &providerError) || calls != 1 || providerError.StatusCode != 400 || providerError.ProviderName != "Groq" {
		t.Fatalf("provider error metadata missing: error=%v calls=%d", err, calls)
	}
	if providerError.Message != "Provider returned error: [redacted]" || providerError.Detail != "Invalid language en [redacted]" {
		t.Fatalf("unexpected bounded diagnostics: %+v", providerError)
	}
	if strings.Contains(err.Error(), "language") || strings.Contains(err.Error(), "secret") || strings.Contains(providerError.Detail, "must not expose") {
		t.Fatal("ordinary error or diagnostics exposed raw provider data")
	}
}

func TestTranscriptionProviderErrorBounds(t *testing.T) {
	body, err := json.Marshal(map[string]any{"error": map[string]any{"message": strings.Repeat("x", 900)}})
	if err != nil {
		t.Fatal(err)
	}
	providerError := decodeTranscriptionProviderError(400, body, "")
	if len([]rune(providerError.Message)) != 513 {
		t.Fatalf("provider message was not bounded: %d characters", len([]rune(providerError.Message)))
	}
	if malformed := decodeTranscriptionProviderError(400, []byte(`not JSON`), ""); malformed.Message != "" || malformed.StatusCode != 400 {
		t.Fatalf("malformed provider body was retained: %+v", malformed)
	}
}

func TestTranscribeMultipartCapIncludesEnvelope(t *testing.T) {
	for _, size := range []int64{maxMultipartBytes, maxMultipartBytes - 1} {
		t.Run(strconv.FormatInt(size, 10), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "large.flac")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(size); err != nil {
				file.Close()
				t.Fatal(err)
			}
			file.Close()
			c := New("test-key", "model")
			c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("oversized multipart request reached provider")
				return nil, errors.New("unexpected provider call")
			})}
			if _, err := c.Transcribe(context.Background(), path); err == nil || !strings.Contains(err.Error(), "25 MB") {
				t.Fatalf("expected multipart cap error, got %v", err)
			}
		})
	}
}

func TestTranscribeResponseSizeLimit(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, body := range []string{
		strings.Repeat(" ", maxTranscriptionBytes+1),
		`{"text":"","segments":[]}` + strings.Repeat(" ", maxTranscriptionBytes),
	} {
		calls := 0
		c := New("test-key", "model")
		c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return providerResponse(http.StatusOK, body), nil
		})}
		if _, err := c.Transcribe(context.Background(), path); err == nil || !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("expected response limit error, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("oversized response retried: %d calls", calls)
		}
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestTranscribeStatusRetriesAndBodyReplay(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, tc := range []struct {
		status int
		calls  int
		ok     bool
	}{
		{http.StatusRequestTimeout, 3, true},
		{http.StatusTooManyRequests, 3, true},
		{http.StatusInternalServerError, 3, true},
		{http.StatusBadGateway, 3, true},
		{http.StatusUnauthorized, 1, false},
		{http.StatusForbidden, 1, false},
		{http.StatusBadRequest, 1, false},
		{http.StatusRequestEntityTooLarge, 1, false},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			calls := 0
			var firstBody []byte
			var responseBodies []*trackedBody
			c := New("test-key", "model")
			c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				r.Body.Close()
				if calls == 1 {
					firstBody = body
				} else if !bytes.Equal(firstBody, body) {
					t.Fatal("retried multipart body changed")
				}
				status, content := tc.status, "provider detail must not leak"
				if tc.ok && calls == 3 {
					status, content = http.StatusOK, validTranscription
				}
				responseBody := &trackedBody{Reader: strings.NewReader(content)}
				responseBodies = append(responseBodies, responseBody)
				return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}}, Body: responseBody}, nil
			})}
			_, err := c.Transcribe(context.Background(), path)
			if (err == nil) != tc.ok || calls != tc.calls {
				t.Fatalf("error=%v, calls=%d, expected success=%t calls=%d", err, calls, tc.ok, tc.calls)
			}
			if err != nil && strings.Contains(err.Error(), "must not leak") {
				t.Fatal("provider error body leaked")
			}
			for _, body := range responseBodies {
				if !body.closed {
					t.Fatal("response body was not closed")
				}
			}
		})
	}
}

type transientNetworkError struct{}

func (transientNetworkError) Error() string   { return "temporary network interruption" }
func (transientNetworkError) Timeout() bool   { return true }
func (transientNetworkError) Temporary() bool { return true }

func TestTranscribeNetworkRetriesAreBounded(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, tc := range []struct {
		name  string
		err   error
		calls int
	}{
		{"network timeout", transientNetworkError{}, 3},
		{"connection EOF", io.EOF, 3},
		{"connection reset", syscall.ECONNRESET, 3},
		{"temporary DNS failure", &net.DNSError{Err: "temporary DNS failure", IsTemporary: true}, 3},
		{"terminal error", errors.New("invalid transport configuration"), 1},
		{"cancellation", context.Canceled, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := New("test-key", "model")
			c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				r.Body.Close()
				return nil, tc.err
			})}
			if _, err := c.Transcribe(context.Background(), path); err == nil {
				t.Fatal("expected transport error")
			}
			if calls != tc.calls {
				t.Fatalf("got %d calls, expected %d", calls, tc.calls)
			}
		})
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestTranscribeInterruptedResponseRetries(t *testing.T) {
	path, _ := transcriptionFile(t)
	calls := 0
	var interruptedBodies []*trackedBody
	c := New("test-key", "model")
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 3 {
			return providerResponse(http.StatusOK, validTranscription), nil
		}
		body := &trackedBody{Reader: io.MultiReader(strings.NewReader(`{"text":"`), errorReader{err: io.ErrUnexpectedEOF})}
		interruptedBodies = append(interruptedBodies, body)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Retry-After": {"0"}}, Body: body}, nil
	})}
	if _, err := c.Transcribe(context.Background(), path); err != nil || calls != 3 {
		t.Fatalf("interrupted response retry failed: error=%v calls=%d", err, calls)
	}
	for _, body := range interruptedBodies {
		if !body.closed {
			t.Fatal("interrupted response body was not closed")
		}
	}
}

func TestTranscribeExhaustedStatusRetries(t *testing.T) {
	path, _ := transcriptionFile(t)
	calls := 0
	c := New("test-key", "model")
	c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		calls++
		resp := providerResponse(http.StatusServiceUnavailable, "")
		resp.Header.Set("Retry-After", "0")
		return resp, nil
	})}
	if _, err := c.Transcribe(context.Background(), path); err == nil || calls != 3 {
		t.Fatalf("expected final status error after three attempts: error=%v calls=%d", err, calls)
	}
}

func TestProviderRetryAfterSecondsAndDate(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"3", 3 * time.Second},
		{" 0 ", 0},
		{now.Add(5 * time.Second).Format(http.TimeFormat), 5 * time.Second},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"-1", 250 * time.Millisecond},
		{"malformed", 250 * time.Millisecond},
		{"9223372036854775807", time.Duration(1<<63 - 1)},
	} {
		if got := providerRetryDelay(tc.value, 250*time.Millisecond, now); got != tc.want {
			t.Fatalf("Retry-After %q: got %v, expected %v", tc.value, got, tc.want)
		}
	}
}

func TestTranscribeRetryAfterCancellation(t *testing.T) {
	path, _ := transcriptionFile(t)
	for _, retryAfter := range []string{"60", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		body := &trackedBody{Reader: strings.NewReader("")}
		c := New("test-key", "model")
		c.http = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			calls++
			time.AfterFunc(10*time.Millisecond, cancel)
			return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {retryAfter}}, Body: body}, nil
		})}
		started := time.Now()
		_, err := c.Transcribe(ctx, path)
		cancel()
		if !errors.Is(err, context.Canceled) || calls != 1 || !body.closed || time.Since(started) > time.Second {
			t.Fatalf("Retry-After cancellation failed: error=%v calls=%d closed=%t", err, calls, body.closed)
		}
	}
}

func TestCompletionUsesSharedTransientRetry(t *testing.T) {
	calls := 0
	var firstBody []byte
	c := New("test-key", "configured/model")
	c.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if calls == 1 {
			firstBody = body
			resp := providerResponse(http.StatusTooManyRequests, "")
			resp.Header.Set("Retry-After", "0")
			return resp, nil
		}
		if !bytes.Equal(firstBody, body) {
			t.Fatal("completion retry body changed")
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil || payload["model"] != "configured/model" {
			t.Fatalf("completion model changed: %v, %v", payload, err)
		}
		return providerResponse(http.StatusOK, `{"model":"configured/model","choices":[{"finish_reason":"stop","message":{"content":"{\"windows\":[]}"}}]}`), nil
	})}
	if _, err := c.Complete(context.Background(), "system", "input"); err != nil || calls != 2 {
		t.Fatalf("completion retry failed: error=%v calls=%d", err, calls)
	}
}
