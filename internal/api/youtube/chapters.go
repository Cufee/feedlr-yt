package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cufee/feedlr-yt/internal/metrics"
)

// Chapter times are seconds, matching the media element's playback clock.
type Chapter struct {
	Start      float64            `json:"start"`
	End        float64            `json:"end"`
	Title      string             `json:"title"`
	Thumbnails []ChapterThumbnail `json:"thumbnails,omitempty"`
}

type ChapterThumbnail struct {
	URL    string                `json:"url"`
	Width  int                   `json:"width"`
	Height int                   `json:"height"`
	Sprite *ChapterThumbnailCrop `json:"sprite,omitempty"`
}

type ChapterThumbnailCrop struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

var chapterVideoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var initialPlayerAssignment = regexp.MustCompile(`(?:var\s+ytInitialPlayerResponse|window\["ytInitialPlayerResponse"\]|ytInitialPlayerResponse)\s*=\s*`)
var descriptionChapterLine = regexp.MustCompile(`^\s*(\d{1,3}:\d{2}(?::\d{2})?)\s+[-–—|]?\s*(\S.*)$`)

// GetVideoChapters fetches the public watch page through the configured YouTube
// transport. It never uses account credentials or consumes Data API quota.
func (c *client) GetVideoChapters(ctx context.Context, videoID string, duration float64) (chapters []Chapter, err error) {
	defer func() { metrics.ObserveYouTubeAPICall("web", "get_video_chapters", err) }()
	if !chapterVideoID.MatchString(videoID) {
		return nil, fmt.Errorf("invalid YouTube video ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.youtube.com/watch?v="+videoID+"&hl=en", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	res, err := c.httpClientWithTimeout(8 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("watch page returned HTTP %d", res.StatusCode)
	}
	const maxPageBytes = 8 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, maxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPageBytes {
		return nil, fmt.Errorf("watch page exceeds size limit")
	}
	return parseChapterPage(strings.NewReader(string(body)), videoID, duration)
}

// GetChapterThumbnail only accepts YouTube's image host, including redirects.
// Callers obtain the URL from cached chapters rather than request parameters.
func (c *client) GetChapterThumbnail(ctx context.Context, imageURL string) ([]byte, string, error) {
	allowed := func(raw string) bool {
		parsed, err := url.Parse(raw)
		return err == nil && parsed.Scheme == "https" && parsed.Host == "i.ytimg.com" && parsed.User == nil
	}
	if !allowed(imageURL) {
		return nil, "", fmt.Errorf("invalid chapter thumbnail host")
	}
	client := c.httpClientWithTimeout(5 * time.Second)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !allowed(req.URL.String()) {
			return fmt.Errorf("invalid chapter thumbnail redirect")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("thumbnail returned HTTP %d", res.StatusCode)
	}
	const maxBytes = 2 << 20
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil || len(body) > maxBytes {
		return nil, "", fmt.Errorf("thumbnail exceeds size limit or failed to read")
	}
	contentType := http.DetectContentType(body)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return nil, "", fmt.Errorf("invalid thumbnail content type")
	}
	return body, contentType, nil
}

type chapterText struct {
	SimpleText string `json:"simpleText"`
	Runs       []struct {
		Text string `json:"text"`
	} `json:"runs"`
}

func (t chapterText) text() string {
	if t.SimpleText != "" {
		return t.SimpleText
	}
	var s strings.Builder
	for _, run := range t.Runs {
		s.WriteString(run.Text)
	}
	return s.String()
}

func parseChapterPage(page io.Reader, videoID string, duration float64) ([]Chapter, error) {
	doc, err := goquery.NewDocumentFromReader(page)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	var player struct {
		Details struct {
			VideoID     string `json:"videoId"`
			Duration    string `json:"lengthSeconds"`
			Description string `json:"shortDescription"`
			Thumbnail   struct {
				Images []ChapterThumbnail `json:"thumbnails"`
			} `json:"thumbnail"`
		} `json:"videoDetails"`
		Storyboards struct {
			Renderer struct {
				Spec string `json:"spec"`
			} `json:"playerStoryboardSpecRenderer"`
		} `json:"storyboards"`
	}
	var playerFound bool
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		text := script.Text()
		if data == nil {
			if match := initialDataAssignment.FindStringIndex(text); match != nil {
				decoder := json.NewDecoder(strings.NewReader(text[match[1]:]))
				decoder.UseNumber()
				var candidate map[string]any
				if decoder.Decode(&candidate) == nil {
					data = candidate
				}
			}
		}
		if !playerFound {
			if match := initialPlayerAssignment.FindStringIndex(text); match != nil {
				playerFound = json.NewDecoder(strings.NewReader(text[match[1]:])).Decode(&player) == nil
				if !playerFound {
					player.Details.Description = ""
				}
			}
		}
	})
	if playerFound && player.Details.VideoID != "" && player.Details.VideoID != videoID {
		return nil, fmt.Errorf("watch page contains another video")
	}
	if playerFound {
		if seconds, err := strconv.ParseFloat(player.Details.Duration, 64); err == nil && seconds > 0 {
			duration = seconds
		}
	}
	if data == nil && !playerFound {
		return nil, fmt.Errorf("watch page contains no metadata")
	}
	previews := func(chapters []Chapter) []Chapter {
		// YouTube can reuse its static video cover in every chapter renderer.
		// Use it for the opening chapter, where the first storyboard frame can
		// be black. Later chapters should show their own time-specific frames.
		covers := make(map[string]bool)
		var opening ChapterThumbnail
		for _, image := range player.Details.Thumbnail.Images {
			if strings.HasPrefix(image.URL, "//") {
				image.URL = "https:" + image.URL
			}
			if parsed, err := url.Parse(image.URL); err == nil {
				covers[parsed.Host+parsed.Path] = true
				if chapterStoryboardURL(parsed) && image.Width > 0 && image.Height > 0 && (opening.URL == "" || storyboardLevelPreferred(image.Width, opening.Width)) {
					opening = image
					opening.Sprite = nil
				}
			}
		}
		for i := range chapters {
			var images []ChapterThumbnail
			for _, image := range chapters[i].Thumbnails {
				parsed, err := url.Parse(image.URL)
				if err == nil && !covers[parsed.Host+parsed.Path] {
					images = append(images, image)
				}
			}
			chapters[i].Thumbnails = images
		}
		chapters = chapterStoryboardThumbnails(chapters, player.Storyboards.Renderer.Spec, duration)
		if len(chapters) > 0 && chapters[0].Start == 0 && opening.URL != "" {
			chapters[0].Thumbnails = []ChapterThumbnail{opening}
		}
		return chapters
	}

	// Restrict extraction to player markers and the dedicated chapters panel.
	// A recursive renderer search also finds unrelated recommended videos.
	markers, _ := chapterPath(data, "playerOverlays", "playerOverlayRenderer", "decoratedPlayerBarRenderer", "decoratedPlayerBarRenderer", "playerBar", "multiMarkersPlayerBarRenderer", "markersMap").([]any)
	for _, key := range []string{"DESCRIPTION_CHAPTERS", "AUTO_CHAPTERS"} {
		for _, marker := range markers {
			if chapterPath(marker, "key") != key {
				continue
			}
			items, _ := chapterPath(marker, "value", "chapters").([]any)
			var chapters []Chapter
			for _, item := range items {
				var renderer struct {
					Title      chapterText `json:"title"`
					Start      *float64    `json:"timeRangeStartMillis"`
					Thumbnails struct {
						Images []ChapterThumbnail `json:"thumbnails"`
					} `json:"thumbnail"`
				}
				if decodeChapterValue(chapterPath(item, "chapterRenderer"), &renderer) && renderer.Start != nil {
					chapters = append(chapters, Chapter{Start: *renderer.Start / 1000, Title: renderer.Title.text(), Thumbnails: renderer.Thumbnails.Images})
				}
			}
			if normalized := normalizeChapters(chapters, duration); len(normalized) > 0 {
				return previews(normalized), nil
			}
		}
	}
	panels, _ := data["engagementPanels"].([]any)
	for _, panel := range panels {
		target, _ := chapterPath(panel, "engagementPanelSectionListRenderer", "targetId").(string)
		if target != "engagement-panel-macro-markers-description-chapters" && target != "engagement-panel-macro-markers-auto-chapters" {
			continue
		}
		items, _ := chapterPath(panel, "engagementPanelSectionListRenderer", "content", "macroMarkersListRenderer", "contents").([]any)
		var chapters []Chapter
		for _, item := range items {
			var renderer struct {
				Title chapterText `json:"title"`
				OnTap struct {
					Watch struct {
						VideoID string   `json:"videoId"`
						Start   *float64 `json:"startTimeSeconds"`
					} `json:"watchEndpoint"`
				} `json:"onTap"`
				Thumbnails struct {
					Images []ChapterThumbnail `json:"thumbnails"`
				} `json:"thumbnail"`
			}
			if decodeChapterValue(chapterPath(item, "macroMarkersListItemRenderer"), &renderer) && renderer.OnTap.Watch.Start != nil && renderer.OnTap.Watch.VideoID == videoID {
				chapters = append(chapters, Chapter{Start: *renderer.OnTap.Watch.Start, Title: renderer.Title.text(), Thumbnails: renderer.Thumbnails.Images})
			}
		}
		if normalized := normalizeChapters(chapters, duration); len(normalized) > 0 {
			return previews(normalized), nil
		}
	}
	return previews(ChaptersFromDescription(player.Details.Description, duration)), nil
}

func chapterPath(value any, keys ...string) any {
	for _, key := range keys {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

func decodeChapterValue(value any, target any) bool {
	if value == nil {
		return false
	}
	encoded, err := json.Marshal(value)
	return err == nil && json.Unmarshal(encoded, target) == nil
}

func normalizeChapters(chapters []Chapter, duration float64) []Chapter {
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return nil
	}
	var normalized []Chapter
	for _, chapter := range chapters {
		chapter.Title = strings.TrimSpace(chapter.Title)
		if chapter.Title == "" || math.IsNaN(chapter.Start) || math.IsInf(chapter.Start, 0) || chapter.Start < 0 || chapter.Start >= duration {
			continue
		}
		if len(normalized) > 0 && chapter.Start <= normalized[len(normalized)-1].Start {
			continue
		}
		var thumbnails []ChapterThumbnail
		for _, image := range chapter.Thumbnails {
			if strings.HasPrefix(image.URL, "//") {
				image.URL = "https:" + image.URL
			}
			if strings.HasPrefix(image.URL, "https://") && image.Width > 0 && image.Height > 0 {
				thumbnails = append(thumbnails, image)
			}
		}
		chapter.Thumbnails = thumbnails
		normalized = append(normalized, chapter)
		if len(normalized) >= 500 {
			break
		}
	}
	for i := range normalized {
		normalized[i].End = duration
		if i+1 < len(normalized) {
			normalized[i].End = normalized[i+1].Start
		}
	}
	return normalized
}

// ChaptersFromDescription accepts only a contiguous timestamp list matching
// YouTube's manual chapter rules: first at zero, at least three, ten seconds each.
func ChaptersFromDescription(description string, duration float64) []Chapter {
	var candidates []Chapter
	for line := range strings.SplitSeq(description, "\n") {
		match := descriptionChapterLine.FindStringSubmatch(line)
		if match == nil {
			if len(candidates) > 0 {
				if result := descriptionChapterBlock(candidates, duration); result != nil {
					return result
				}
				candidates = nil
			}
			continue
		}
		parts := strings.Split(match[1], ":")
		seconds := 0
		valid := true
		for i, part := range parts {
			n, _ := strconv.Atoi(part)
			if i > 0 && n >= 60 {
				valid = false
			}
			seconds = seconds*60 + n
		}
		if !valid {
			candidates = nil
			continue
		}
		candidates = append(candidates, Chapter{Start: float64(seconds), Title: match[2]})
	}
	return descriptionChapterBlock(candidates, duration)
}

func descriptionChapterBlock(chapters []Chapter, duration float64) []Chapter {
	if len(chapters) < 3 || chapters[0].Start != 0 {
		return nil
	}
	for i, chapter := range chapters {
		end := duration
		if i+1 < len(chapters) {
			end = chapters[i+1].Start
		}
		if end-chapter.Start < 10 {
			return nil
		}
	}
	return normalizeChapters(chapters, duration)
}
