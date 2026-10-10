package youtube

import (
	"math"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const storyboardTemplate = "https://i.ytimg.com/sb/abcdefghijk/storyboard3_L$L/$N.jpg?keep=value&sigh=old"

func TestChapterPageOpeningCoverAndHighResolutionStoryboard(t *testing.T) {
	page := chapterWatchFixture(`[{"key":"DESCRIPTION_CHAPTERS","value":{"chapters":[
		{"chapterRenderer":{"title":{"simpleText":"Intro"},"timeRangeStartMillis":0}},
		{"chapterRenderer":{"title":{"simpleText":"Middle"},"timeRangeStartMillis":20000}},
		{"chapterRenderer":{"title":{"simpleText":"End"},"timeRangeStartMillis":40000}}
	]}}]`)
	page += `<script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abcdefghijk","lengthSeconds":"60","thumbnail":{"thumbnails":[
		{"url":"https://i.ytimg.com/vi/abcdefghijk/cover-small.jpg","width":168,"height":94},
		{"url":"https://i.ytimg.com/vi/abcdefghijk/cover-medium.jpg","width":336,"height":188},
		{"url":"https://i.ytimg.com/vi/abcdefghijk/cover-large.jpg","width":1920,"height":1080}
	]}},"storyboards":{"playerStoryboardSpecRenderer":{"spec":"` + storyboardTemplate + `|160#90#6#3#2#10000#M$M#small|320#180#6#3#3#10000#M$M#large"}}};</script>`
	chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 0)
	if err != nil || len(chapters) != 3 {
		t.Fatalf("unexpected parsed chapters: %+v, %v", chapters, err)
	}
	first := chapters[0].Thumbnails
	if len(first) != 1 || !strings.Contains(first[0].URL, "cover-medium.jpg") || first[0].Sprite != nil || first[0].Width != 336 || first[0].Height != 188 {
		t.Fatalf("opening preview should use a suitably sized video cover: %+v", first)
	}
	for i, crop := range []ChapterThumbnailCrop{{X: 640, Y: 0, Width: 320, Height: 180}, {X: 320, Y: 180, Width: 320, Height: 180}} {
		thumb := chapters[i+1].Thumbnails[0]
		if !strings.Contains(thumb.URL, "storyboard3_L1/M0.jpg") || !reflect.DeepEqual(thumb.Sprite, &crop) {
			t.Fatalf("later chapter must retain its high resolution frame: %+v %+v", thumb, thumb.Sprite)
		}
	}
}

func TestChapterPageStoryboardReplacesRepeatedCoverWithDistinctCrops(t *testing.T) {
	page := chapterWatchFixture(`[{"key":"DESCRIPTION_CHAPTERS","value":{"chapters":[
		{"chapterRenderer":{"title":{"simpleText":"Intro"},"timeRangeStartMillis":0,"thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg","width":168,"height":94}]}}},
		{"chapterRenderer":{"title":{"simpleText":"Middle"},"timeRangeStartMillis":20000,"thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg","width":168,"height":94}]}}},
		{"chapterRenderer":{"title":{"simpleText":"End"},"timeRangeStartMillis":40000,"thumbnail":{"thumbnails":[{"url":"https://i.ytimg.com/vi/abcdefghijk/hqdefault.jpg","width":168,"height":94}]}}}
	]}}]`)
	page += `<script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abcdefghijk","lengthSeconds":"60"},"storyboards":{"playerStoryboardSpecRenderer":{"spec":"` + storyboardTemplate + `|160#90#6#3#2#10000#M$M#rs$signature"}}};</script>`
	chapters, err := parseChapterPage(strings.NewReader(page), "abcdefghijk", 0)
	if err != nil || len(chapters) != 3 {
		t.Fatalf("unexpected parsed chapters: %+v, %v", chapters, err)
	}
	for i, crop := range []ChapterThumbnailCrop{
		{X: 0, Y: 0, Width: 160, Height: 90},
		{X: 320, Y: 0, Width: 160, Height: 90},
		{X: 160, Y: 90, Width: 160, Height: 90},
	} {
		thumb := chapters[i].Thumbnails[0]
		if !strings.Contains(thumb.URL, "/storyboard3_L0/M0.jpg") || !reflect.DeepEqual(thumb.Sprite, &crop) || thumb.Width != 480 || thumb.Height != 180 {
			t.Fatalf("chapter %d retained cover or wrong crop: %+v %+v", i, thumb, thumb.Sprite)
		}
	}
	if chapters[0].Title != "Intro" || chapters[2].End != 60 {
		t.Fatal("storyboards changed chapter metadata")
	}
}

func TestChapterStoryboardFramesAndSheetBoundaries(t *testing.T) {
	spec := storyboardTemplate + "|48#27#100#10#10#0#default#rs$small|160#90#61#5#5#2000#M$M#rs$medium"
	chapters := []Chapter{{Start: 0}, {Start: 49.9}, {Start: 50}, {Start: 120}, {Start: 10000}}
	got := chapterStoryboardThumbnails(chapters, spec, 150)
	for i, tc := range []struct {
		sheet, x, y int
	}{
		{0, 0, 0}, {0, 640, 360}, {1, 0, 0}, {2, 0, 180}, {2, 0, 180},
	} {
		if len(got[i].Thumbnails) != 1 {
			t.Fatalf("chapter %d missing thumbnail", i)
		}
		thumb := got[i].Thumbnails[0]
		image, err := url.Parse(thumb.URL)
		if err != nil {
			t.Fatal(err)
		}
		wantPath := "/sb/abcdefghijk/storyboard3_L1/M" + string(rune('0'+tc.sheet)) + ".jpg"
		if image.Path != wantPath || image.Query().Get("sigh") != "rs$medium" || image.Query().Get("keep") != "value" {
			t.Fatalf("chapter %d incorrect sheet URL: %s", i, image)
		}
		if thumb.Width != 800 || thumb.Height != 450 || !reflect.DeepEqual(thumb.Sprite, &ChapterThumbnailCrop{X: tc.x, Y: tc.y, Width: 160, Height: 90}) {
			t.Fatalf("chapter %d incorrect crop: %+v %+v", i, thumb, thumb.Sprite)
		}
	}
}

func TestChapterStoryboardCombinedSheetInfersInterval(t *testing.T) {
	spec := storyboardTemplate + "|48#27#100#10#10#0#default#rs$combined"
	got := chapterStoryboardThumbnails([]Chapter{{Start: 0}, {Start: 1}, {Start: 99.9}, {Start: 100}}, spec, 100)
	for i, crop := range []ChapterThumbnailCrop{
		{X: 0, Y: 0, Width: 48, Height: 27},
		{X: 48, Y: 0, Width: 48, Height: 27},
		{X: 432, Y: 243, Width: 48, Height: 27},
		{X: 432, Y: 243, Width: 48, Height: 27},
	} {
		thumb := got[i].Thumbnails[0]
		if !strings.Contains(thumb.URL, "/storyboard3_L0/default.jpg") || !reflect.DeepEqual(thumb.Sprite, &crop) {
			t.Fatalf("chapter %d incorrect inferred frame: %+v %+v", i, thumb, thumb.Sprite)
		}
	}
}

func TestChapterStoryboardChoosesLargestSmallLevelOrSmallestUsefulLevel(t *testing.T) {
	for _, tc := range []struct {
		levels string
		width  int
		index  string
	}{
		{"|48#27#10#5#2#1000#M$M#small|96#54#10#5#2#1000#M$M#medium", 96, "L1"},
		{"|320#180#10#3#3#1000#M$M#large|160#90#10#5#2#1000#M$M#medium", 320, "L0"},
		{"|160#90#10#5#2#1000#M$M#medium|320#180#10#3#3#1000#M$M#large", 320, "L1"},
		{"|640#360#10#3#3#1000#M$M#huge|320#180#10#3#3#1000#M$M#large", 320, "L1"},
		{"|broken|160#90#10#5#2#1000#M$M#medium", 160, "L1"},
	} {
		got := chapterStoryboardThumbnails([]Chapter{{Start: 0}}, storyboardTemplate+tc.levels, 30)
		thumb := got[0].Thumbnails[0]
		if thumb.Sprite.Width != tc.width || !strings.Contains(thumb.URL, "storyboard3_"+tc.index) {
			t.Fatalf("unexpected chosen level: %+v %+v", thumb, thumb.Sprite)
		}
	}
}

func TestChapterStoryboardMalformedSpecPreservesExistingThumbnail(t *testing.T) {
	validLevel := "|160#90#10#5#2#1000#M$M#rs$signature"
	for _, spec := range []string{
		"", "invalid", "https://i.ytimg.com/sb/a.jpg",
		"http://i.ytimg.com/sb/$N.jpg" + validLevel,
		"https://i.ytimg.com.evil.test/sb/$N.jpg" + validLevel,
		"https://user@i.ytimg.com/sb/$N.jpg" + validLevel,
		"https://i.ytimg.com:443/sb/$N.jpg" + validLevel,
		"https://i.ytimg.com/sb/$N.jpg#fragment" + validLevel,
		storyboardTemplate + "|0#90#10#5#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#0#5#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#10#0#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#100001#5#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#10#101#2#1000#M$M#sig",
		storyboardTemplate + "|2049#90#10#5#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#10#5#2#-1#M$M#sig",
		storyboardTemplate + "|160#90#10#5#2#86400001#M$M#sig",
		storyboardTemplate + "|160#90#10#5#2#1000#M$M#",
		storyboardTemplate + "|160#90#10#5#2#1000#M$M#sig#extra",
		storyboardTemplate + "|999999999999999999999999#90#10#5#2#1000#M$M#sig",
		storyboardTemplate + "|160#90#11#5#2#1000#default#sig",
		storyboardTemplate + "|2048#2048#10#100#100#1000#M$M#sig",
		storyboardTemplate + strings.Repeat("|160#90#10#5#2#1000#M$M#sig", 33),
		strings.Repeat("x", 65537),
	} {
		original := []Chapter{{Start: 5, Thumbnails: []ChapterThumbnail{{URL: "https://i.ytimg.com/cover.jpg", Width: 168, Height: 94}}}}
		got := chapterStoryboardThumbnails(original, spec, 60)
		if !reflect.DeepEqual(got[0].Thumbnails, []ChapterThumbnail{{URL: "https://i.ytimg.com/cover.jpg", Width: 168, Height: 94}}) {
			t.Fatalf("malformed spec replaced original: %q", spec)
		}
	}
}

func TestChapterStoryboardNonFiniteTimesDoNotPanic(t *testing.T) {
	spec := storyboardTemplate + "|160#90#10#5#2#1000#M$M#sig"
	for _, duration := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		got := chapterStoryboardThumbnails([]Chapter{{Start: 0}}, spec, duration)
		if len(got[0].Thumbnails) != 0 {
			t.Fatal("invalid duration generated storyboard")
		}
	}
	for _, start := range []float64{-1, math.NaN(), math.Inf(1)} {
		got := chapterStoryboardThumbnails([]Chapter{{Start: start}}, spec, 60)
		if len(got[0].Thumbnails) != 0 {
			t.Fatal("invalid start generated storyboard")
		}
	}
}
