package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// TranscriptionModel is independent of the model used to classify sponsor windows.
const TranscriptionModel = "openai/whisper-large-v3-turbo"

const (
	transcriptionEndpoint = "https://openrouter.ai/api/v1/audio/transcriptions"
	maxMultipartBytes     = 25_000_000
	maxTranscriptionBytes = 2 << 20
	providerAttempts      = 3
)

type TranscriptionSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type TranscriptionResult struct {
	Text         string                 `json:"text"`
	Language     string                 `json:"language,omitempty"`
	Duration     float64                `json:"duration"`
	Segments     []TranscriptionSegment `json:"segments"`
	Usage        json.RawMessage        `json:"usage,omitempty"`
	GenerationID string                 `json:"generation_id,omitempty"`
}

// TranscriptionProviderError exposes bounded diagnostics without including a
// provider's raw response in ordinary error logs.
type TranscriptionProviderError struct {
	StatusCode   int
	ProviderName string
	Message      string
	Detail       string
}

func (e *TranscriptionProviderError) Error() string {
	return fmt.Sprintf("transcription provider returned status %d", e.StatusCode)
}

// Transcribe uploads one FLAC chunk and requests timestamps in seconds. The
// caller owns chunk duration validation and the shared provider concurrency limit.
func (c *Client) Transcribe(ctx context.Context, path string) (TranscriptionResult, error) {
	return c.TranscribeWithLanguage(ctx, path, "")
}

// TranscribeWithLanguage optionally fixes the transcription language using a
// two-letter ISO 639-1 code. An empty language asks the provider to detect it.
func (c *Client) TranscribeWithLanguage(ctx context.Context, path, language string) (TranscriptionResult, error) {
	if err := ctx.Err(); err != nil {
		return TranscriptionResult{}, err
	}
	language = strings.TrimSpace(language)
	if language != "" && len(language) != 2 {
		return TranscriptionResult{}, errors.New("transcription language must be a two-letter ISO 639-1 code")
	}
	for _, letter := range language {
		if !(letter >= 'a' && letter <= 'z' || letter >= 'A' && letter <= 'Z') {
			return TranscriptionResult{}, errors.New("transcription language must be a two-letter ISO 639-1 code")
		}
	}
	language = strings.ToLower(language)
	file, err := os.Open(path)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("open transcription audio: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("stat transcription audio: %w", err)
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 {
		return TranscriptionResult{}, errors.New("transcription audio must be a nonempty regular FLAC file")
	}
	if stat.Size() >= maxMultipartBytes {
		return TranscriptionResult{}, errors.New("transcription multipart body must be below 25 MB")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&multipartLimitWriter{buffer: &body})
	for _, field := range [][2]string{
		{"model", TranscriptionModel},
		{"response_format", "verbose_json"},
		{"timestamp_granularities[]", "segment"},
	} {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return TranscriptionResult{}, err
		}
	}
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return TranscriptionResult{}, err
		}
	}
	part, err := writer.CreatePart(textproto.MIMEHeader{
		// OpenRouter's multipart parser requires quoted disposition values.
		// A fixed FLAC filename also avoids path-specific header encoding.
		"Content-Disposition": {`form-data; name="file"; filename="audio.flac"`},
		"Content-Type":        {"audio/flac"},
	})
	if err != nil {
		return TranscriptionResult{}, err
	}
	if _, err := io.Copy(part, io.LimitReader(file, maxMultipartBytes)); err != nil {
		return TranscriptionResult{}, fmt.Errorf("build transcription multipart body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return TranscriptionResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transcriptionEndpoint, bytes.NewReader(body.Bytes()))
	if err != nil {
		return TranscriptionResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.doWithRetry(req)
	if err != nil {
		return TranscriptionResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return TranscriptionResult{}, decodeTranscriptionProviderError(resp.StatusCode, body, c.key)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTranscriptionBytes+1))
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("read transcription response: %w", err)
	}
	if len(responseBody) > maxTranscriptionBytes {
		return TranscriptionResult{}, errors.New("transcription response exceeded size limit")
	}
	result, err := decodeTranscription(responseBody)
	if err != nil {
		return TranscriptionResult{}, err
	}
	result.GenerationID = resp.Header.Get("X-Generation-Id")
	return result, nil
}

func decodeTranscriptionProviderError(status int, body []byte, key string) *TranscriptionProviderError {
	result := &TranscriptionProviderError{StatusCode: status}
	var decoded struct {
		Error struct {
			Message  string `json:"message"`
			Metadata struct {
				ProviderName string `json:"provider_name"`
				Raw          string `json:"raw"`
			} `json:"metadata"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &decoded) != nil {
		return result
	}
	clean := func(value string) string {
		if key != "" {
			value = strings.ReplaceAll(value, key, "[redacted]")
		}
		value = strings.Join(strings.Fields(value), " ")
		letters := []rune(value)
		if len(letters) > 512 {
			value = string(letters[:512]) + "…"
		}
		return value
	}
	result.ProviderName = clean(decoded.Error.Metadata.ProviderName)
	result.Message = clean(decoded.Error.Message)
	// Some providers wrap their useful validation message in a JSON string.
	// Keep the message alone instead of exposing the rest of that payload.
	var provider struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(decoded.Error.Metadata.Raw), &provider) == nil {
		switch {
		case provider.Error.Message != "":
			result.Detail = clean(provider.Error.Message)
		case provider.Message != "":
			result.Detail = clean(provider.Message)
		case provider.Detail != "":
			result.Detail = clean(provider.Detail)
		}
	}
	return result
}

// multipartLimitWriter bounds the entire multipart envelope, including metadata
// and closing boundaries, even if the file changes after the initial stat.
type multipartLimitWriter struct {
	buffer *bytes.Buffer
}

func (w *multipartLimitWriter) Write(p []byte) (int, error) {
	if len(p) >= maxMultipartBytes-w.buffer.Len() {
		return 0, errors.New("transcription multipart body must be below 25 MB")
	}
	return w.buffer.Write(p)
}

func decodeTranscription(body []byte) (TranscriptionResult, error) {
	var decoded struct {
		Text     *string         `json:"text"`
		Language string          `json:"language"`
		Duration json.RawMessage `json:"duration"`
		Segments *[]struct {
			Start *float64 `json:"start"`
			End   *float64 `json:"end"`
			Text  *string  `json:"text"`
		} `json:"segments"`
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return TranscriptionResult{}, fmt.Errorf("decode transcription response: %w", err)
	}
	if decoded.Text == nil || decoded.Segments == nil {
		return TranscriptionResult{}, errors.New("transcription response omitted text or timestamp segments")
	}
	result := TranscriptionResult{Text: *decoded.Text, Language: decoded.Language, Segments: make([]TranscriptionSegment, 0, len(*decoded.Segments)), Usage: decoded.Usage}
	if len(decoded.Duration) != 0 {
		var duration *float64
		if err := json.Unmarshal(decoded.Duration, &duration); err != nil || duration == nil || !validSeconds(*duration) {
			return TranscriptionResult{}, errors.New("transcription response contained invalid duration")
		}
		result.Duration = *duration
	}
	if len(*decoded.Segments) == 0 && strings.TrimSpace(result.Text) != "" {
		return TranscriptionResult{}, errors.New("transcription response omitted timestamps for nonempty text")
	}
	previousEnd := 0.0
	for _, segment := range *decoded.Segments {
		if segment.Start == nil || segment.End == nil || segment.Text == nil ||
			!validSeconds(*segment.Start) || !validSeconds(*segment.End) ||
			*segment.End <= *segment.Start || *segment.Start < previousEnd {
			return TranscriptionResult{}, errors.New("transcription response contained invalid or unordered timestamp segment")
		}
		previousEnd = *segment.End
		if strings.TrimSpace(*segment.Text) == "" {
			if strings.TrimSpace(result.Text) != "" {
				return TranscriptionResult{}, errors.New("transcription response contained an empty timestamp segment for nonempty text")
			}
			// Whisper providers may describe silence with an empty-text interval.
			// Normalize those intervals to the same result as explicit [] segments.
			continue
		}
		result.Segments = append(result.Segments, TranscriptionSegment{Start: *segment.Start, End: *segment.End, Text: *segment.Text})
	}
	if len(result.Segments) != 0 && strings.TrimSpace(result.Text) == "" {
		return TranscriptionResult{}, errors.New("transcription response contained segments without text")
	}
	return result, nil
}

func validSeconds(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}

// doWithRetry recreates the request body on each attempt. Responses are retried
// only before decoding, so malformed provider output never triggers another call.
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	for attempt := range providerAttempts {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		current := req
		if attempt > 0 {
			current = req.Clone(req.Context())
			if req.GetBody == nil {
				return nil, errors.New("provider request body cannot be replayed")
			}
			var err error
			current.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
		}
		resp, err := c.http.Do(current)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			// Transport errors can also occur while reading a successful response.
			// Bound that read before returning a replayable body to the decoder.
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxTranscriptionBytes+1))
			resp.Body.Close()
			if readErr != nil {
				err = fmt.Errorf("read provider response: %w", readErr)
			} else {
				resp.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		if req.Context().Err() != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return nil, req.Context().Err()
		}
		if attempt == providerAttempts-1 || (err == nil && !retryableStatus(resp.StatusCode)) || (err != nil && !retryableNetworkError(err)) {
			return resp, err
		}
		delay := time.Duration(1<<attempt) * 250 * time.Millisecond
		if resp != nil {
			delay = providerRetryDelay(resp.Header.Get("Retry-After"), delay, time.Now())
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
	panic("unreachable provider retry loop")
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500 && status <= 599
}

func retryableNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return true
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) && dnsError.IsTemporary {
		return true
	}
	for _, code := range []syscall.Errno{syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.EPIPE, syscall.EHOSTUNREACH, syscall.ENETUNREACH, syscall.EINTR} {
		if errors.Is(err, code) {
			return true
		}
	}
	return false
}

func providerRetryDelay(value string, fallback time.Duration, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 {
		maxSeconds := int64(math.MaxInt64) / int64(time.Second)
		if seconds > maxSeconds {
			return time.Duration(math.MaxInt64)
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		if date.After(now) {
			return date.Sub(now)
		}
		return 0
	}
	return fallback
}
