package youtube

import (
	"math"
	"net/url"
	"strconv"
	"strings"
)

type chapterStoryboardLevel struct {
	index                int
	width, height, count int
	columns, rows        int
	interval             float64
	name, signature      string
}

// chapterStoryboardThumbnails maps chapter starts to individual frames in a
// YouTube storyboard sheet. A missing or malformed spec preserves thumbnails.
func chapterStoryboardThumbnails(chapters []Chapter, spec string, duration float64) []Chapter {
	if len(chapters) == 0 || len(spec) == 0 || len(spec) > 65536 || duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return chapters
	}
	parts := strings.Split(spec, "|")
	if len(parts) < 2 || len(parts) > 33 {
		return chapters
	}
	base, err := url.Parse(parts[0])
	if err != nil || !chapterStoryboardURL(base) {
		return chapters
	}
	var selected *chapterStoryboardLevel
	for i, raw := range parts[1:] {
		level, ok := parseChapterStoryboardLevel(raw, i, duration)
		if !ok {
			continue
		}
		if selected == nil || storyboardLevelPreferred(level.width, selected.width) {
			copy := level
			selected = &copy
		}
	}
	if selected == nil {
		return chapters
	}
	level := *selected
	template := strings.ReplaceAll(strings.ReplaceAll(parts[0], "$L", strconv.Itoa(level.index)), "$N", level.name)
	if level.count > level.columns*level.rows && !strings.Contains(template, "$M") {
		return chapters
	}
	for i := range chapters {
		start := chapters[i].Start
		if start < 0 || math.IsNaN(start) || math.IsInf(start, 0) {
			continue
		}
		frame := math.Floor(start / level.interval)
		// Clamp as floats before conversion, including unusually large starts.
		frame = math.Min(frame, float64(level.count-1))
		index := int(frame)
		capacity := level.columns * level.rows
		sheet := index / capacity
		image, err := url.Parse(strings.ReplaceAll(template, "$M", strconv.Itoa(sheet)))
		if err != nil || !chapterStoryboardURL(image) {
			continue
		}
		query := image.Query()
		// The entire signature is required, including YouTube's "rs$" prefix.
		query.Set("sigh", level.signature)
		image.RawQuery = query.Encode()
		cell := index % capacity
		chapters[i].Thumbnails = []ChapterThumbnail{{
			URL: image.String(), Width: level.width * level.columns, Height: level.height * level.rows,
			Sprite: &ChapterThumbnailCrop{X: (cell % level.columns) * level.width, Y: (cell / level.columns) * level.height, Width: level.width, Height: level.height},
		}}
	}
	return chapters
}

func chapterStoryboardURL(parsed *url.URL) bool {
	return parsed != nil && parsed.Scheme == "https" && parsed.Host == "i.ytimg.com" && parsed.User == nil && parsed.Fragment == ""
}

func storyboardLevelPreferred(candidate, selected int) bool {
	// A 320px frame stays sharp in the 160px scrub preview on a 2x display.
	// Choose the smallest adequate level to keep sheet downloads inexpensive.
	if candidate >= 320 {
		return selected < 320 || candidate < selected
	}
	return selected < 320 && candidate > selected
}

func parseChapterStoryboardLevel(raw string, index int, duration float64) (chapterStoryboardLevel, bool) {
	fields := strings.Split(raw, "#")
	if len(fields) != 8 || fields[6] == "" || len(fields[6]) > 128 || fields[7] == "" || len(fields[7]) > 4096 {
		return chapterStoryboardLevel{}, false
	}
	var numbers [6]int
	for i := range numbers {
		value, err := strconv.Atoi(fields[i])
		if err != nil || value < 0 {
			return chapterStoryboardLevel{}, false
		}
		numbers[i] = value
	}
	width, height, count, columns, rows, interval := numbers[0], numbers[1], numbers[2], numbers[3], numbers[4], numbers[5]
	if width < 1 || width > 2048 || height < 1 || height > 2048 || count < 1 || count > 100000 || columns < 1 || columns > 100 || rows < 1 || rows > 100 || interval > 86400000 {
		return chapterStoryboardLevel{}, false
	}
	if width*columns > 16384 || height*rows > 16384 || width*columns*height*rows > 32<<20 {
		return chapterStoryboardLevel{}, false
	}
	seconds := float64(interval) / 1000
	if interval == 0 {
		seconds = duration / float64(count)
	}
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return chapterStoryboardLevel{}, false
	}
	return chapterStoryboardLevel{index: index, width: width, height: height, count: count, columns: columns, rows: rows, interval: seconds, name: fields[6], signature: fields[7]}, true
}
