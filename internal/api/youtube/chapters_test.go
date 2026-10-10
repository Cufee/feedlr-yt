package youtube

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Mirrors the watch-page player marker path without retaining tracking data.
func chapterWatchFixture(markers string) string {
	return `<script>var ytInitialData = {"playerOverlays":{"playerOverlayRenderer":{"decoratedPlayerBarRenderer":{"decoratedPlayerBarRenderer":{"playerBar":{"multiMarkersPlayerBarRenderer":{"markersMap":` + markers + `}}}}}}}; afterData();</script>`
}

func TestParseChapterPagePrefersStructuredAndNormalizes(t *testing.T) {
	page := chapterWatchFixture(`[{"key":"DESCRIPTION_CHAPTERS","value":{"chapters":[
		{"chapterRenderer":{"title":{"simpleText":" Intro "},"timeRangeStartMillis":0,"thumbnail":{"thumbnails":[{"url":"//i.ytimg.com/vi/abcdefghijk/0.jpg","width":168,"height":94}]}}},
		{"chapterRenderer":{"title":{"runs":[{"text":"Part "},{"text":"two"}]},"timeRangeStartMillis":12500}},
		{"chapterRenderer":{"title":{"simpleText":"Duplicate"},"timeRangeStartMillis":12500}},
		{"chapterRenderer":{"title":{"simpleText":"Reverse"},"timeRangeStartMillis":10000}},
		{"chapterRenderer":{"title":{"simpleText":"Final"},"timeRangeStartMillis":30000}},
		{"chapterRenderer":{"title":{"simpleText":"Beyond duration"},"timeRangeStartMillis":99000}}
	]}}]`) + `<script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abcdefghijk","lengthSeconds":"60","shortDescription":"0:00 Wrong\n0:20 Fallback\n0:40 Titles"}};</script>`
	chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 3 || chapters[0].Title != "Intro" || chapters[0].End != 12.5 || chapters[1].Title != "Part two" || chapters[2].End != 60 || chapters[0].Thumbnails[0].URL != "https://i.ytimg.com/vi/abcdefghijk/0.jpg" {
		t.Fatalf("unexpected chapters: %+v", chapters)
	}
}

func TestParseChapterPageAutomaticAndPanelFallback(t *testing.T) {
	for _, page := range []string{
		chapterWatchFixture(`[{"key":"AUTO_CHAPTERS","value":{"chapters":[{"chapterRenderer":{"title":{"simpleText":"Automatic"},"timeRangeStartMillis":0}}]}}]`),
		`<script>window["ytInitialData"] = {"engagementPanels":[{"engagementPanelSectionListRenderer":{"targetId":"engagement-panel-macro-markers-auto-chapters","content":{"macroMarkersListRenderer":{"contents":[{"macroMarkersListItemRenderer":{"title":{"simpleText":"Automatic"},"onTap":{"watchEndpoint":{"videoId":"abcdefghijk","startTimeSeconds":0}}}},{"macroMarkersListItemRenderer":{"title":{"simpleText":"Another video"},"onTap":{"watchEndpoint":{"videoId":"another_id_","startTimeSeconds":20}}}}]}}}}]};</script>`,
	} {
		chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 60)
		if err != nil || len(chapters) != 1 || chapters[0].Title != "Automatic" || chapters[0].End != 60 {
			t.Fatalf("unexpected chapters: %+v, %v", chapters, err)
		}
	}
}

func TestChapterPageRejectsMismatchAndMissingMetadata(t *testing.T) {
	for _, page := range []string{
		`<html>Before you continue</html>`,
		`<script>var ytInitialData = {broken};</script>`,
		`<script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"another_id_","lengthSeconds":"60"}};</script>`,
	} {
		if _, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 60); err == nil {
			t.Fatalf("accepted bad page: %s", page)
		}
	}
	page := `<script>var ytInitialData = {"recommendedVideo":{"chapterRenderer":{"title":{"simpleText":"Unrelated"},"timeRangeStartMillis":0}}};</script>`
	if chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 60); err != nil || len(chapters) != 0 {
		t.Fatalf("included unrelated chapters: %+v, %v", chapters, err)
	}
}

func TestChaptersFromDescriptionConservativeFallback(t *testing.T) {
	for _, tc := range []struct {
		description string
		duration    float64
		count       int
	}{
		{"Intro text\n0:00 Intro\n0:20 – Middle\n0:40 End\nLinks below", 60, 3},
		{"0:00 Intro\n1:20:00 Middle\n2:00:00 End", 8000, 3},
		{"0:00 Intro\n0:20 Middle", 60, 0},
		{"0:01 Intro\n0:20 Middle\n0:40 End", 60, 0},
		{"0:00 Intro\n0:05 Middle\n0:40 End", 60, 0},
		{"0:00 Intro\n0:40 Middle\n0:20 End", 60, 0},
		{"0:00 Intro\n0:20 Middle\n0:40 End", 45, 0},
		{"0:00 Intro\n0:60 Middle\n2:00 End", 180, 0},
		{"See 0:00 and 0:20 references\n0:40 End", 60, 0},
		{"0:00 Intro\nText in between\n0:20 Middle\n0:40 End", 60, 0},
	} {
		chapters := ChaptersFromDescription(tc.description, tc.duration)
		if len(chapters) != tc.count {
			t.Fatalf("%q: got %+v, want %d", tc.description, chapters, tc.count)
		}
	}
}

func TestGetChaptersUsesPublicTransportAndBodyLimit(t *testing.T) {
	calls := 0
	c := &client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Host != "www.youtube.com" || r.URL.Path != "/watch" || r.URL.Query().Get("v") != "abcdefghijk" || r.Header.Get("Authorization") != "" || r.URL.Query().Get("key") != "" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		body := chapterWatchFixture(`[]`)
		if calls > 1 {
			body = strings.Repeat("x", (8<<20)+1)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	if _, err := c.GetVideoChapters(context.Background(), "abcdefghijk", 60); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetVideoChapters(context.Background(), "abcdefghijk", 60); err == nil {
		t.Fatal("accepted oversized page")
	}
	if _, err := c.GetVideoChapters(context.Background(), "../../bad", 60); err == nil || calls != 2 {
		t.Fatal("invalid video ID made request")
	}
}

func TestChapterThumbnailRejectsHostsAndNonImages(t *testing.T) {
	calls := 0
	c := &client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<html>not an image</html>")), Header: make(http.Header)}, nil
	})}}
	for _, image := range []string{"http://i.ytimg.com/0.jpg", "https://localhost/0.jpg", "https://i.ytimg.com.evil.test/0.jpg", "https://i.ytimg.com:443/0.jpg", "https://user@i.ytimg.com/0.jpg"} {
		if _, _, err := c.GetChapterThumbnail(context.Background(), image); err == nil {
			t.Fatalf("accepted URL %s", image)
		}
	}
	if calls != 0 {
		t.Fatal("untrusted thumbnail made request")
	}
	if _, _, err := c.GetChapterThumbnail(context.Background(), "https://i.ytimg.com/0.jpg"); err == nil {
		t.Fatal("accepted non-image response")
	}
}

func TestChapterThumbnailValidImageRedirectAndSizeLimits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		redirect string
		wantErr  bool
	}{
		{name: "image", body: "\xff\xd8\xffsample jpeg"},
		{name: "oversized", body: "\xff\xd8\xff" + strings.Repeat("x", 2<<20), wantErr: true},
		{name: "redirect outside host", redirect: "https://example.com/0.jpg", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := &client{http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "i.ytimg.com" {
					t.Fatalf("followed external redirect to %s", r.URL)
				}
				header := make(http.Header)
				status := http.StatusOK
				if tc.redirect != "" {
					status = http.StatusFound
					header.Set("Location", tc.redirect)
				}
				return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			body, contentType, err := c.GetChapterThumbnail(context.Background(), "https://i.ytimg.com/0.jpg")
			if (err != nil) != tc.wantErr || calls != 1 {
				t.Fatalf("unexpected result: %v, calls %d", err, calls)
			}
			if !tc.wantErr && (string(body) != tc.body || contentType != "image/jpeg") {
				t.Fatalf("unexpected image: %q, %s", body, contentType)
			}
		})
	}
}

func TestChapterJSONUsesSecondsAndLowercaseNames(t *testing.T) {
	encoded, err := json.Marshal(Chapter{Start: 12.5, End: 20, Title: "Part", Thumbnails: []ChapterThumbnail{{URL: "https://i.ytimg.com/0.jpg", Width: 168, Height: 94}}})
	if err != nil || !strings.Contains(string(encoded), `"start":12.5`) || !strings.Contains(string(encoded), `"url":`) {
		t.Fatalf("unexpected JSON: %s, %v", encoded, err)
	}
}

func TestChapterPreviewsUseCoverOnlyAtStartAndRetainActualFrames(t *testing.T) {
	page := chapterWatchFixture(`[{"key":"DESCRIPTION_CHAPTERS","value":{"chapters":[
		{"chapterRenderer":{"title":{"simpleText":"Cover placeholder"},"timeRangeStartMillis":0,"thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg?size=large","width":336,"height":188}]}}},
		{"chapterRenderer":{"title":{"simpleText":"Real frame"},"timeRangeStartMillis":20000,"thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/frame.jpg","width":336,"height":188}]}}}
	]}}]`) + `<script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abcdefghijk","lengthSeconds":"60","thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg?size=small","width":168,"height":94}]}}};</script>`
	chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 60)
	if err != nil || len(chapters) != 2 {
		t.Fatalf("unexpected chapters: %+v, %v", chapters, err)
	}
	if len(chapters[0].Thumbnails) != 1 || chapters[0].Thumbnails[0].Width != 168 || len(chapters[1].Thumbnails) != 1 || !strings.Contains(chapters[1].Thumbnails[0].URL, "/frame.jpg") {
		t.Fatalf("opening cover or later chapter frame missing: %+v", chapters)
	}
}
