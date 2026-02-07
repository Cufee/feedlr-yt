package auth

import (
	"bytes"
	"context"
	"encoding/json"

	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func (c *Client) GetContext(ctx context.Context) (*WebPlayerRequestContext, error) {
	if c.context != nil {
		return c.context, nil
	}
	context, err := c.newWebPlayerRequestContext()
	if err != nil {
		return nil, err
	}
	c.context = context
	return c.context, nil
}

func (c *Client) newWebPlayerRequestContext() (*WebPlayerRequestContext, error) {
	req, err := http.NewRequest("GET", youtubeBaseURL+"/sw.js_data", nil)
	if err != nil {
		return nil, err
	}

	tz, tzOffset := time.Now().Zone()
	context := WebPlayerRequestContext{
		VisitorID: randomString(11),
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Referer", youtubeBaseURL+"/sw.js")
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	req.Header.Set("Cookie", fmt.Sprintf("PREF=tz=%s;VISITOR_INFO1_LIVE=%s", strings.ReplaceAll(tz, "/", "."), context.VisitorID))

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("bad status code returned from /sw.js_data")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(string(body), ")]}'") {
		return nil, errors.New("invalid JSPB response")
	}
	cleanBody := strings.TrimSpace(strings.TrimPrefix(string(body), ")]}'"))

	var bodyData [][]any
	err = json.Unmarshal([]byte(cleanBody), &bodyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse /sw.js_data body")
	}

	if len(bodyData) < 1 || len(bodyData[0]) < 3 {
		return nil, errors.New("invalid body data received from /sw.js_data")
	}
	config, ok := bodyData[0][2].([]any)
	if !ok || len(config) < 2 {
		return nil, errors.New("invalid config received from /sw.js_data")
	}

	// get api key
	context.ApiKey, ok = config[1].(string)
	if !ok {
		return nil, errors.New("invalid api key received from /sw.js_data")
	}

	// get device info
	deviceInfoSlice, ok := config[0].([]any)
	if !ok || len(deviceInfoSlice) < 1 {
		return nil, errors.New("invalid deviceInfoSlice received from /sw.js_data")
	}

	deviceInfo, ok := deviceInfoSlice[0].([]any)
	if !ok || len(deviceInfoSlice) < 1 {
		return nil, errors.New("invalid deviceInfo received from /sw.js_data")
	}

	clientOverwrites := map[string]any{
		"timeZone":         tz,
		"remoteHost":       deviceInfo[3],
		"visitorData":      deviceInfo[13],
		"clientVersion":    deviceInfo[16],
		"utcOffsetMinutes": int64(-tzOffset / 60),
	}

	configInfo, ok := deviceInfo[61].([]any)
	if ok && len(configInfo) > 0 {
		clientOverwrites["configInfo"] = map[string]any{
			"appInstallData": configInfo[len(configInfo)-1],
		}
	}

	context.data = newWebPlayerClient(clientOverwrites)
	return &context, err

}

type WebPlayerRequestContext struct {
	ApiKey    string
	VisitorID string
	data      map[string]any
}

func (c WebPlayerRequestContext) ForVideo(token string, id string) (io.Reader, error) {
	body := newWebPlayerRequest(c.data, id)
	body["continuation"] = token

	encoded, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(encoded), nil
}

func newWebPlayerClient(overwrites map[string]any) map[string]any {
	base := map[string]any{
		"hl":                 "en",
		"gl":                 "US",
		"clientName":         "WEB",
		"platform":           "DESKTOP",
		"browserName":        "Chrome",
		"browserVersion":     "109.0.0.0",
		"screenDensityFloat": 1,
		"screenHeightPoints": 1440,
		"screenPixelDensity": 1,
		"screenWidthPoints":  2560,
		"clientFormFactor":   "UNKNOWN_FORM_FACTOR",
		"userInterfaceTheme": "USER_INTERFACE_THEME_LIGHT",
		"memoryTotalKbytes":  "8000000",
		"originalUrl":        "https://youtube.com",
		"mainAppWebInfo": map[string]any{
			"graftUrl":                  "https://youtube.com",
			"pwaInstallabilityStatus":   "PWA_INSTALLABILITY_STATUS_UNKNOWN",
			"webDisplayMode":            "WEB_DISPLAY_MODE_BROWSER",
			"isWebNativeShareAvailable": true,
		},
	}
	for key, value := range overwrites {
		base[key] = value
	}
	return base
}

func newWebPlayerRequest(client map[string]any, videoID string) map[string]any {
	return map[string]any{
		"videoId": videoID,
		"context": map[string]any{
			"client": client,
			"user": map[string]any{
				"enableSafetyMode": false,
				"lockedSafetyMode": false,
			},
			"request": map[string]any{
				"useSsl":                  true,
				"internalExperimentFlags": []any{},
			},
		},
	}
}

var randomStringRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")

func randomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = randomStringRunes[rand.Intn(len(randomStringRunes))]
	}
	return string(b)
}

var userAgents = []string{
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.3 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36 Edg/109.0.1518.61",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.2 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
}
