package google

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"golang.org/x/exp/maps"
)

type VideoDetails struct {
	IsShort  bool `json:"isShort"`
	Duration int  `json:"duration"`
}

type PlayerResponse struct {
	StreamingData StreamingData `json:"streamingData"`
}

type StreamingData struct {
	ExpiresInSeconds string            `json:"expiresInSeconds"`
	Formats          []Formats         `json:"formats"`
	AdaptiveFormats  []AdaptiveFormats `json:"adaptiveFormats"`
}

type Formats struct {
	Itag             int    `json:"itag"`
	URL              string `json:"url"`
	MimeType         string `json:"mimeType"`
	Bitrate          int    `json:"bitrate"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	LastModified     string `json:"lastModified"`
	ContentLength    string `json:"contentLength,omitempty"`
	Quality          string `json:"quality"`
	Fps              int    `json:"fps"`
	QualityLabel     string `json:"qualityLabel"`
	ProjectionType   string `json:"projectionType"`
	AverageBitrate   int    `json:"averageBitrate,omitempty"`
	AudioQuality     string `json:"audioQuality"`
	ApproxDurationMs string `json:"approxDurationMs"`
	AudioSampleRate  string `json:"audioSampleRate"`
	AudioChannels    int    `json:"audioChannels"`
}

type AdaptiveFormats struct {
	Itag             int     `json:"itag"`
	URL              string  `json:"url"`
	MimeType         string  `json:"mimeType"`
	Bitrate          int     `json:"bitrate"`
	Width            int     `json:"width,omitempty"`
	Height           int     `json:"height,omitempty"`
	LastModified     string  `json:"lastModified"`
	ContentLength    string  `json:"contentLength"`
	Quality          string  `json:"quality"`
	Fps              int     `json:"fps,omitempty"`
	QualityLabel     string  `json:"qualityLabel,omitempty"`
	ProjectionType   string  `json:"projectionType"`
	AverageBitrate   int     `json:"averageBitrate"`
	ApproxDurationMs string  `json:"approxDurationMs"`
	HighReplication  bool    `json:"highReplication,omitempty"`
	AudioQuality     string  `json:"audioQuality,omitempty"`
	AudioSampleRate  string  `json:"audioSampleRate,omitempty"`
	AudioChannels    int     `json:"audioChannels,omitempty"`
	LoudnessDb       float64 `json:"loudnessDb,omitempty"`
}

var defaultClientBodyString = `{"videoId":"","contentCheckOk":true,"racyCheckOk":true,"context":{"client":{"clientName":"WEB","clientVersion":"1.20210616.1.0","platform":"DESKTOP","clientScreen":"EMBED","clientFormFactor":"UNKNOWN_FORM_FACTOR","browserName":"Chrome"},"user":{"lockedSafetyMode":"false"},"request":{"useSsl":"true"}}}`
var defaultClientBody map[string]any

func init() {
	defaultClientBody = make(map[string]any)
	err := json.Unmarshal([]byte(defaultClientBodyString), &defaultClientBody)
	if err != nil {
		panic(err)
	}
}

var playerLimiter = time.NewTicker(time.Second / 15)

func (c *client) GetVideoPlayerDetails(videoId string) (*VideoDetails, error) {
	<-playerLimiter.C

	body := make(map[string]any)
	maps.Copy(body, defaultClientBody)

	body["videoId"] = videoId
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	res, err := http.Post("https://www.youtube.com/youtubei/v1/player", "application/json", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	if res == nil || res.StatusCode != 200 {
		return nil, errors.New("invalid response")
	}

	var details PlayerResponse
	err = json.NewDecoder(res.Body).Decode(&details)
	if err != nil {
		return nil, err
	}

	if len(details.StreamingData.Formats) > 0 {
		duration, _ := strconv.Atoi(details.StreamingData.Formats[0].ApproxDurationMs)
		return &VideoDetails{
			IsShort:  details.StreamingData.Formats[0].Width < details.StreamingData.Formats[0].Height,
			Duration: duration / 1000,
		}, nil
	}

	if len(details.StreamingData.AdaptiveFormats) > 0 {
		duration, _ := strconv.Atoi(details.StreamingData.AdaptiveFormats[0].ApproxDurationMs)
		return &VideoDetails{
			IsShort:  details.StreamingData.AdaptiveFormats[0].Width < details.StreamingData.AdaptiveFormats[0].Height,
			Duration: duration / 1000,
		}, nil
	}

	log.Warnf("no formats found for video %s", videoId)
	return &VideoDetails{
		IsShort:  false,
		Duration: 0,
	}, nil
}
