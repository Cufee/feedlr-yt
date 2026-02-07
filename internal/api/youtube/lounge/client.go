package lounge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
)

const defaultLoungeAPIBaseURL = "https://www.youtube.com/api/lounge"

const (
	defaultDeviceName = "Feedlr TV Sync"
	maxChunkLineSize  = 8 * 1024 * 1024
	requestTimeout    = 12 * time.Second
)

var (
	ErrAuthExpired  = errors.New("lounge auth expired")
	ErrUnknownSID   = errors.New("lounge unknown sid")
	ErrSessionGone  = errors.New("lounge session gone")
	ErrNotConnected = errors.New("lounge is not connected")
)

type Screen struct {
	ID          string
	Name        string
	LoungeToken string
}

type Event struct {
	ID   int64
	Type string
	Args []any
}

type PlaybackEvent struct {
	VideoID string

	State string

	CurrentTime    float64
	Duration       float64
	HasCurrentTime bool
	HasDuration    bool
}

type Session struct {
	ScreenID    string
	LoungeToken string
	DeviceName  string

	SID        string
	GSessionID string
	LastEvent  int64

	commandOffset int64
	mu            sync.Mutex
}

type Client struct {
	http    *http.Client
	baseURL string
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		http:    httpClient,
		baseURL: defaultLoungeAPIBaseURL,
	}
}

func NewClientWithBaseURL(httpClient *http.Client, baseURL string) *Client {
	client := NewClient(httpClient)
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
	return client
}

func (c *Client) PairWithCode(ctx context.Context, pairingCode string) (Screen, error) {
	code := strings.TrimSpace(pairingCode)
	if code == "" {
		err := fmt.Errorf("pairing code is required")
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}

	form := url.Values{}
	form.Set("pairing_code", code)

	reqCtx, cancel := withRequestTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/pairing/get_screen", strings.NewReader(form.Encode()))
	if err != nil {
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeTVCall("pair_with_code_request", err)
	if err != nil {
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("lounge pair failed with status %d", res.StatusCode)
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}

	var payload struct {
		Screen struct {
			ScreenID    string `json:"screenId"`
			Name        string `json:"name"`
			LoungeToken string `json:"loungeToken"`
		} `json:"screen"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}
	if payload.Screen.ScreenID == "" || payload.Screen.LoungeToken == "" {
		err := fmt.Errorf("invalid lounge pair response")
		metrics.ObserveYouTubeTVCall("pair_with_code", err)
		return Screen{}, err
	}

	metrics.ObserveYouTubeTVCall("pair_with_code", nil)
	return Screen{
		ID:          payload.Screen.ScreenID,
		Name:        payload.Screen.Name,
		LoungeToken: payload.Screen.LoungeToken,
	}, nil
}

func (c *Client) RefreshLoungeToken(ctx context.Context, screenID string) (Screen, error) {
	screenID = strings.TrimSpace(screenID)
	if screenID == "" {
		err := errors.New("screen id is required")
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}

	form := url.Values{}
	form.Set("screen_ids", screenID)

	reqCtx, cancel := withRequestTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/pairing/get_lounge_token_batch", strings.NewReader(form.Encode()))
	if err != nil {
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeTVCall("refresh_lounge_token_request", err)
	if err != nil {
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("refresh lounge token failed with status %d", res.StatusCode)
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}

	var payload struct {
		Screens []struct {
			ScreenID    string `json:"screenId"`
			Name        string `json:"name"`
			LoungeToken string `json:"loungeToken"`
		} `json:"screens"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}
	if len(payload.Screens) == 0 {
		err := errors.New("refresh lounge token returned no screens")
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}

	screen := payload.Screens[0]
	if screen.ScreenID == "" || screen.LoungeToken == "" {
		err := errors.New("refresh lounge token response missing fields")
		metrics.ObserveYouTubeTVCall("refresh_lounge_token", err)
		return Screen{}, err
	}

	metrics.ObserveYouTubeTVCall("refresh_lounge_token", nil)
	return Screen{
		ID:          screen.ScreenID,
		Name:        screen.Name,
		LoungeToken: screen.LoungeToken,
	}, nil
}

func (c *Client) Connect(ctx context.Context, screenID, loungeToken, deviceName string) (*Session, error) {
	screenID = strings.TrimSpace(screenID)
	loungeToken = strings.TrimSpace(loungeToken)
	if screenID == "" || loungeToken == "" {
		err := errors.New("screen id and lounge token are required")
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}
	if strings.TrimSpace(deviceName) == "" {
		deviceName = defaultDeviceName
	}

	connectBody := url.Values{}
	connectBody.Set("app", "web")
	connectBody.Set("mdx-version", "3")
	connectBody.Set("name", deviceName)
	connectBody.Set("id", screenID)
	connectBody.Set("device", "REMOTE_CONTROL")
	connectBody.Set("capabilities", "que,dsdtr,atp,vsp")
	connectBody.Set("magnaKey", "cloudPairedDevice")
	connectBody.Set("ui", "false")
	connectBody.Set("deviceContext", "user_agent=feedlr&window_width_points=&window_height_points=&os_name=macos")
	connectBody.Set("theme", "cl")
	connectBody.Set("loungeIdToken", loungeToken)

	connectURL := c.baseURL + "/bc/bind?RID=1&VER=8&CVER=1&auth_failure_option=send_error"
	reqCtx, cancel := withRequestTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, connectURL, strings.NewReader(connectBody.Encode()))
	if err != nil {
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeTVCall("connect_request", err)
	if err != nil {
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if err := sessionResponseError(res.StatusCode, string(body)); err != nil {
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}

	session := &Session{
		ScreenID:      screenID,
		LoungeToken:   loungeToken,
		DeviceName:    deviceName,
		commandOffset: 1,
	}

	err = parseEventChunks(bytes.NewReader(body), func(events []Event) error {
		session.applyEvents(events)
		return nil
	})
	if err != nil {
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}
	if !session.connected() {
		err := errors.New("lounge connect missing session identifiers")
		metrics.ObserveYouTubeTVCall("connect", err)
		return nil, err
	}

	metrics.ObserveYouTubeTVCall("connect", nil)
	return session, nil
}

func (c *Client) Subscribe(ctx context.Context, session *Session, onEvent func(Event) error) error {
	if session == nil || !session.connected() {
		metrics.ObserveYouTubeTVCall("subscribe", ErrNotConnected)
		return ErrNotConnected
	}

	params := session.commonParams()
	params.Set("RID", "rpc")
	params.Set("CI", "0")
	params.Set("TYPE", "xmlhttp")

	url := c.baseURL + "/bc/bind?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		metrics.ObserveYouTubeTVCall("subscribe", err)
		return err
	}

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeTVCall("subscribe_request", err)
	if err != nil {
		if ctx.Err() != nil {
			metrics.ObserveYouTubeTVCall("subscribe", ctx.Err())
			return ctx.Err()
		}
		metrics.ObserveYouTubeTVCall("subscribe", err)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		err := sessionResponseError(res.StatusCode, string(payload))
		metrics.ObserveYouTubeTVCall("subscribe", err)
		return err
	}

	err = parseEventChunks(res.Body, func(events []Event) error {
		session.applyEvents(events)
		for _, event := range events {
			switch event.Type {
			case "c", "S":
				continue
			}
			if onEvent != nil {
				if err := onEvent(event); err != nil {
					return err
				}
			}
		}
		return nil
	})
	metrics.ObserveYouTubeTVCall("subscribe", err)
	return err
}

func (c *Client) SeekTo(ctx context.Context, session *Session, seconds float64) error {
	if seconds < 0 {
		seconds = 0
	}
	return c.command(ctx, session, "seekTo", map[string]string{
		"newTime": strconv.FormatFloat(seconds, 'f', 3, 64),
	})
}

func (c *Client) GetNowPlaying(ctx context.Context, session *Session) error {
	return c.command(ctx, session, "getNowPlaying", nil)
}

func (c *Client) command(ctx context.Context, session *Session, command string, commandParameters map[string]string) error {
	if session == nil || !session.connected() {
		metrics.ObserveYouTubeTVCall("command_"+command, ErrNotConnected)
		return ErrNotConnected
	}

	rid, ofs := session.nextCommandRID()
	params := session.commonParams()
	params.Set("RID", strconv.FormatInt(rid, 10))

	body := url.Values{}
	body.Set("count", "1")
	body.Set("ofs", strconv.FormatInt(ofs, 10))
	body.Set("req0__sc", command)
	for key, value := range commandParameters {
		body.Set("req0_"+key, value)
	}

	reqCtx, cancel := withRequestTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/bc/bind?"+params.Encode(), strings.NewReader(body.Encode()))
	if err != nil {
		metrics.ObserveYouTubeTVCall("command_"+command, err)
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	metrics.ObserveYouTubeTVCall("command_request_"+command, err)
	if err != nil {
		if ctx.Err() != nil {
			metrics.ObserveYouTubeTVCall("command_"+command, ctx.Err())
			return ctx.Err()
		}
		metrics.ObserveYouTubeTVCall("command_"+command, err)
		return err
	}
	defer res.Body.Close()

	payload, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err := sessionResponseError(res.StatusCode, string(payload)); err != nil {
		metrics.ObserveYouTubeTVCall("command_"+command, err)
		return err
	}
	metrics.ObserveYouTubeTVCall("command_"+command, nil)
	return nil
}

func ExtractPlaybackEvent(event Event) (PlaybackEvent, bool) {
	if event.Type != "nowPlaying" && event.Type != "onStateChange" {
		return PlaybackEvent{}, false
	}
	if len(event.Args) == 0 {
		return PlaybackEvent{}, false
	}

	payload, ok := event.Args[0].(map[string]any)
	if !ok {
		return PlaybackEvent{}, false
	}

	playback := PlaybackEvent{}
	if videoID, ok := payload["videoId"]; ok {
		playback.VideoID, _ = videoID.(string)
	}
	if state, ok := payload["state"]; ok {
		playback.State = asString(state)
	}
	if currentTime, ok := parseFloatField(payload["currentTime"]); ok {
		playback.CurrentTime = currentTime
		playback.HasCurrentTime = true
	}
	if duration, ok := parseFloatField(payload["duration"]); ok {
		playback.Duration = duration
		playback.HasDuration = true
	}

	if playback.VideoID == "" && !playback.HasCurrentTime && !playback.HasDuration && playback.State == "" {
		return PlaybackEvent{}, false
	}
	return playback, true
}

func parseFloatField(v any) (float64, bool) {
	s := asString(v)
	if s == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func asString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(value, 10)
	case int:
		return strconv.Itoa(value)
	default:
		return ""
	}
}

func parseEventChunks(r io.Reader, onChunk func(events []Event) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxChunkLineSize)

	remaining := 0
	var chunkBuilder strings.Builder

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if remaining <= 0 {
			if strings.TrimSpace(line) == "" {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				return fmt.Errorf("invalid lounge chunk length %q: %w", line, err)
			}
			remaining = n
			chunkBuilder.Reset()
			continue
		}

		chunkBuilder.WriteString(line)
		remaining -= len(line) + 1
		if remaining > 0 {
			continue
		}
		if remaining < 0 {
			return errors.New("invalid lounge chunk framing")
		}

		events, err := parseEventChunkJSON([]byte(chunkBuilder.String()))
		if err != nil {
			return err
		}
		if onChunk != nil {
			if err := onChunk(events); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if remaining > 0 {
		return errors.New("incomplete lounge chunk")
	}

	return nil
}

func parseEventChunkJSON(chunk []byte) ([]Event, error) {
	if len(chunk) == 0 {
		return nil, nil
	}

	var payload []any
	if err := json.Unmarshal(chunk, &payload); err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(payload))
	for _, raw := range payload {
		eventArray, ok := raw.([]any)
		if !ok || len(eventArray) < 2 {
			continue
		}

		eventID, ok := parseIntField(eventArray[0])
		if !ok {
			continue
		}

		eventTuple, ok := eventArray[1].([]any)
		if !ok || len(eventTuple) == 0 {
			continue
		}

		eventType, ok := eventTuple[0].(string)
		if !ok || eventType == "" {
			continue
		}

		events = append(events, Event{
			ID:   eventID,
			Type: eventType,
			Args: eventTuple[1:],
		})
	}

	return events, nil
}

func parseIntField(v any) (int64, bool) {
	switch value := v.(type) {
	case float64:
		return int64(value), true
	case int64:
		return value, true
	case int:
		return int64(value), true
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func (s *Session) connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.SID != "" && s.GSessionID != ""
}

func (s *Session) applyEvents(events []Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range events {
		s.LastEvent = event.ID
		switch event.Type {
		case "c":
			if len(event.Args) > 0 {
				s.SID = asString(event.Args[0])
			}
		case "S":
			if len(event.Args) > 0 {
				s.GSessionID = asString(event.Args[0])
			}
		}
	}
}

func (s *Session) nextCommandRID() (rid int64, ofs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ofs = s.commandOffset
	s.commandOffset++
	rid = s.commandOffset
	return rid, ofs
}

func (s *Session) commonParams() url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()

	params := url.Values{}
	params.Set("name", s.DeviceName)
	params.Set("loungeIdToken", s.LoungeToken)
	params.Set("SID", s.SID)
	params.Set("AID", strconv.FormatInt(s.LastEvent, 10))
	params.Set("gsessionid", s.GSessionID)
	params.Set("device", "REMOTE_CONTROL")
	params.Set("app", "youtube-desktop")
	params.Set("VER", "8")
	params.Set("v", "2")
	return params
}

func sessionResponseError(statusCode int, body string) error {
	if statusCode == http.StatusOK {
		return nil
	}

	if statusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: %s", ErrAuthExpired, strings.TrimSpace(body))
	}
	if statusCode == http.StatusBadRequest && strings.Contains(body, "Unknown SID") {
		return fmt.Errorf("%w: %s", ErrUnknownSID, strings.TrimSpace(body))
	}
	if statusCode == http.StatusGone && strings.Contains(body, "Gone") {
		return fmt.Errorf("%w: %s", ErrSessionGone, strings.TrimSpace(body))
	}

	trimmedBody := strings.TrimSpace(body)
	if len(trimmedBody) > 200 {
		trimmedBody = trimmedBody[:200]
	}
	return fmt.Errorf("lounge request failed with status %d: %s", statusCode, trimmedBody)
}

func withRequestTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
