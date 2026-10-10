package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
)

func TestPodcastAudioChunksAndMerge(t *testing.T) {
	repeatedChunks, _ := podcastAudioChunks(600000)
	repeated, err := mergePodcastTranscription(repeatedChunks, []openrouter.TranscriptionResult{{Segments: []openrouter.TranscriptionSegment{{Start: 0, End: 1, Text: "Yes"}, {Start: 2, End: 3, Text: "Yes"}}}}, 600000)
	if err != nil || len(repeated) != 2 {
		t.Fatalf("legitimate repeated speech was removed: %+v %v", repeated, err)
	}
	for _, duration := range []int{1, 600000, 3600000, 7200000, maxPodcastDurationMS} {
		chunks, err := podcastAudioChunks(duration)
		if err != nil || len(chunks) != (duration+podcastChunkMS-1)/podcastChunkMS {
			t.Fatalf("chunks %d: %+v %v", duration, chunks, err)
		}
		for i, chunk := range chunks {
			if chunk.EndMS-chunk.StartMS > podcastChunkMS+2000 || chunk.StartMS != max(0, i*podcastChunkMS-2000) {
				t.Fatalf("chunk: %+v", chunk)
			}
		}
	}
	for _, duration := range []int{0, -1, maxPodcastDurationMS + 1} {
		if _, err := podcastAudioChunks(duration); err == nil {
			t.Fatalf("accepted duration %d", duration)
		}
	}
	chunks, _ := podcastAudioChunks(1200000)
	results := []openrouter.TranscriptionResult{
		{Segments: []openrouter.TranscriptionSegment{{Start: 1, End: 2, Text: "Opening"}, {Start: 598.5, End: 600, Text: "Boundary phrase"}}},
		{Segments: []openrouter.TranscriptionSegment{{Start: .5, End: 2.5, Text: "Boundary phrase"}, {Start: 2.5, End: 4, Text: "Continue"}}},
	}
	cues, err := mergePodcastTranscription(chunks, results, 1200000)
	if err != nil || len(cues) != 3 || cues[2].StartMS != 600500 || cues[2].EndMS != 602000 || cues[2].Index != 2 {
		t.Fatalf("offset/duplicate merge: %+v %v", cues, err)
	}
	for _, segment := range []openrouter.TranscriptionSegment{{Start: 0, End: math.NaN(), Text: "bad"}, {Start: 0, End: math.Inf(1), Text: "bad"}, {Start: 0, End: 601, Text: "bad"}, {Start: -1, End: 1, Text: "bad"}} {
		bad := []openrouter.TranscriptionResult{{Segments: []openrouter.TranscriptionSegment{segment}}, {}}
		if _, err := mergePodcastTranscription(chunks, bad, 1200000); err == nil {
			t.Fatalf("accepted malformed: %+v", segment)
		}
	}
	silent := []openrouter.TranscriptionResult{{Segments: []openrouter.TranscriptionSegment{}}, {Segments: []openrouter.TranscriptionSegment{}}}
	if cues, err := mergePodcastTranscription(chunks, silent, 1200000); err != nil || len(cues) != 0 {
		t.Fatalf("silence: %+v %v", cues, err)
	}
}

func TestPodcastMergePreservesOverlappingSegments(t *testing.T) {
	chunks, err := podcastAudioChunks(1200000)
	if err != nil {
		t.Fatal(err)
	}
	results := []openrouter.TranscriptionResult{
		{},
		{Segments: []openrouter.TranscriptionSegment{
			{Start: 294.51, End: 295.01, Text: "Hello"},
			{Start: 294.63, End: 303.15, Text: "world"},
		}},
	}
	cues, err := mergePodcastTranscription(chunks, results, 1200000)
	if err != nil {
		t.Fatal(err)
	}
	want := []transcriptCue{
		{Index: 0, StartMS: 892510, EndMS: 893010, Text: "Hello"},
		{Index: 1, StartMS: 892630, EndMS: 901150, Text: "world"},
	}
	if len(cues) != len(want) {
		t.Fatalf("overlapping cues lost: %+v", cues)
	}
	for i, cue := range cues {
		if cue != want[i] {
			t.Fatalf("cue %d changed: got=%+v want=%+v", i, cue, want[i])
		}
	}
}

func TestPodcastDownloadBoundsAndRetry(t *testing.T) {
	for _, mode := range []string{"known oversized", "stream oversized", "empty", "retry", "terminal"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch mode {
				case "retry":
					if calls < 3 {
						w.Header().Set("Retry-After", "0")
						w.WriteHeader(429)
						return
					}
					w.Write([]byte("audio"))
				case "terminal":
					w.WriteHeader(403)
				case "empty":
					return
				case "stream oversized":
					w.(http.Flusher).Flush()
					w.Write([]byte(strings.Repeat("a", 101)))
				default:
					w.Write([]byte(strings.Repeat("a", 101)))
				}
			}))
			defer host.Close()
			path := filepath.Join(t.TempDir(), "audio")
			hash, err := downloadPodcastAudio(context.Background(), host.URL, path, 100)
			if mode == "retry" {
				if err != nil || len(hash) != 64 || calls != 3 {
					t.Fatalf("retry: %s %v calls=%d", hash, err, calls)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("bound/terminal: %v calls=%d", err, calls)
			}
		})
	}
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(503) }))
	defer host.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := downloadPodcastAudio(ctx, host.URL, filepath.Join(t.TempDir(), "audio"), 100); err != context.DeadlineExceeded {
		t.Fatalf("retry cancellation: %v", err)
	}
}

func TestPodcastFFmpegChunkEncodingAndOffsets(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; runtime chunk conversion is not covered")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed; runtime duration probe is not covered")
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "input.flac")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "anullsrc=r=22050:cl=stereo", "-t", "602", "-c:a", "flac", input).CombinedOutput(); err != nil {
		t.Fatalf("audio fixture: %s %v", output, err)
	}
	duration, err := probePodcastDuration(ctx, input)
	if err != nil || math.Abs(float64(duration-602000)) > 10 {
		t.Fatalf("probe: %d %v", duration, err)
	}
	chunks, _ := podcastAudioChunks(duration)
	if len(chunks) != 2 {
		t.Fatalf("chunks: %+v", chunks)
	}
	for i, chunk := range chunks {
		chunk.Path = filepath.Join(dir, string(rune('a'+i))+".flac")
		if err = convertPodcastChunk(ctx, input, chunk); err != nil {
			t.Fatal(err)
		}
		output, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "stream=sample_rate,channels,bits_per_raw_sample:format=duration", "-of", "json", chunk.Path).Output()
		if err != nil {
			t.Fatal(err)
		}
		var info struct {
			Streams []struct {
				Rate     string `json:"sample_rate"`
				Channels int    `json:"channels"`
				Bits     string `json:"bits_per_raw_sample"`
			} `json:"streams"`
		}
		if err = json.Unmarshal(output, &info); err != nil || len(info.Streams) != 1 || info.Streams[0].Rate != "16000" || info.Streams[0].Channels != 1 || info.Streams[0].Bits != "16" {
			t.Fatalf("encoding: %s %v", output, err)
		}
		chunkDuration, err := probePodcastDuration(ctx, chunk.Path)
		if err != nil || math.Abs(float64(chunkDuration-(chunk.EndMS-chunk.StartMS))) > 10 {
			t.Fatalf("chunk offset/duration: %d %v", chunkDuration, err)
		}
		stat, err := os.Stat(chunk.Path)
		if err != nil || stat.Size() >= 25_000_000 {
			t.Fatalf("upload bound: %v", err)
		}
	}
}

func TestPodcastChunksRetainOpeningLanguage(t *testing.T) {
	chunks, _ := podcastAudioChunks(3600000)
	for i := range chunks {
		chunks[i].Path = fmt.Sprint(i)
	}
	var openingComplete atomic.Bool
	var active, maxActive atomic.Int32
	var calls atomic.Int32
	results, err := transcribePodcastAudioChunks(context.Background(), chunks, func(context.Context, podcastAudioChunk) error { return nil }, func(ctx context.Context, path, language string) (openrouter.TranscriptionResult, error) {
		calls.Add(1)
		if path == "0" {
			if language != "" {
				t.Error("opening should auto detect language")
			}
			openingComplete.Store(true)
			return openrouter.TranscriptionResult{Language: "English"}, nil
		}
		if !openingComplete.Load() || language != "en" {
			t.Errorf("later chunk lost opening language: %s", language)
		}
		count := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); count > old && !maxActive.CompareAndSwap(old, count); old = maxActive.Load() {
		}
		time.Sleep(10 * time.Millisecond)
		return openrouter.TranscriptionResult{Language: language}, nil
	})
	if err != nil || len(results) != 6 || calls.Load() != 6 || maxActive.Load() > 4 || results[5].Language != "en" {
		t.Fatalf("language/concurrency: results=%+v err=%v calls=%d active=%d", results, err, calls.Load(), maxActive.Load())
	}
	for raw, want := range map[string]string{"English": "en", "en-US": "en", "ru": "ru", "Russian": "ru", "": "", "unknown": ""} {
		if got := normalizedTranscriptionLanguage(raw); got != want {
			t.Errorf("%q => %q, want %q", raw, got, want)
		}
	}
}
