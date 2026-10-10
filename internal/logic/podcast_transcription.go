package logic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/openrouter"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/lucsky/cuid"
	"golang.org/x/sync/errgroup"
)

const maxPodcastDownloadBytes int64 = 512 << 20
const podcastChunkMS = 10 * 60 * 1000
const podcastOverlapMS = 2000

type podcastAudioChunk struct {
	StartMS, EndMS, CoreStartMS int
	Path                        string
}

func podcastAudioChunks(durationMS int) ([]podcastAudioChunk, error) {
	if durationMS <= 0 || durationMS > maxPodcastDurationMS {
		return nil, processingError("episode_too_long_or_invalid", nil)
	}
	var chunks []podcastAudioChunk
	for core := 0; core < durationMS; core += podcastChunkMS {
		chunks = append(chunks, podcastAudioChunk{StartMS: max(0, core-podcastOverlapMS), CoreStartMS: core, EndMS: min(durationMS, core+podcastChunkMS)})
	}
	return chunks, nil
}

func generatePodcastTranscript(ctx context.Context, db database.Client, source podcastInput) (database.PodcastTranscriptContent, error) {
	return generatePodcastTranscriptChunks(ctx, db, source, nil)
}

func generatePodcastTranscriptChunks(ctx context.Context, db database.Client, source podcastInput, ready func(podcastTranscriptChunk) error) (database.PodcastTranscriptContent, error) {
	v := source.video
	if openrouter.DefaultClient == nil {
		return database.PodcastTranscriptContent{}, processingError("provider_disabled", nil)
	}
	dir, err := os.MkdirTemp("", "feedlr-podcast-")
	if err != nil {
		return database.PodcastTranscriptContent{}, err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "enclosure")
	started := time.Now()
	var identity podcastSourceIdentity
	audioHash, err := downloadPodcastAudio(ctx, v.MediaURL.String, input, maxPodcastDownloadBytes, &identity)
	outcome := "ready"
	if err != nil {
		outcome = "failed"
	}
	metrics.ObservePodcastProcessingStage("download", outcome, time.Since(started).Seconds())
	if err != nil {
		return database.PodcastTranscriptContent{}, processingError("audio_download_failed", err)
	}
	identity.SampleHash, err = hashPodcastFileSamples(input, identity.Length)
	if err != nil {
		return database.PodcastTranscriptContent{}, processingError("audio_download_failed", err)
	}
	sources := podcastValidationInputs(source)
	if !podcastRenditionMatches(sources.Audio, identity) {
		// A source that selects new ads for every GET must not create an endless
		// job/download chain. Fail before any STT charge; a later playback POST
		// explicitly revalidates and may retry the source.
		return database.PodcastTranscriptContent{}, processingError("source_changed", nil)
	}
	durationMS, err := probePodcastDuration(ctx, input)
	if err != nil {
		return database.PodcastTranscriptContent{}, err
	}
	chunks, err := podcastAudioChunks(durationMS)
	if err != nil {
		return database.PodcastTranscriptContent{}, err
	}
	started = time.Now()
	for i := range chunks {
		chunks[i].Path = filepath.Join(dir, fmt.Sprintf("chunk-%02d.flac", i))
	}
	chunkByPath := make(map[string]int, len(chunks))
	for i, chunk := range chunks {
		chunkByPath[chunk.Path] = i
	}
	provenance, _ := json.Marshal(map[string]any{"audio_sha256": audioHash, "downloaded_audio": identity, "chunk_ms": podcastChunkMS, "overlap_ms": podcastOverlapMS, "sample_rate": 16000, "channels": 1, "sample_bits": 16})
	cacheKey := func(i int) string {
		return podcastSHA256([]byte(fmt.Sprintf("%s:%s:chunk-v1:%d", source.transcriptKey, audioHash, i)))
	}
	loadChunk := func(i int) (openrouter.TranscriptionResult, bool, error) {
		content, err := db.GetPodcastTranscriptContent(ctx, v.ID, cacheKey(i))
		if errors.Is(err, sql.ErrNoRows) {
			return openrouter.TranscriptionResult{}, false, nil
		}
		if err != nil {
			return openrouter.TranscriptionResult{}, false, err
		}
		var details podcastChunkInputs
		var cues []transcriptCue
		if json.Unmarshal(content.InputJSON, &details) != nil || details.SourceKey != source.transcriptKey || details.AudioHash != audioHash || details.Index != i || details.StartMS != chunks[i].StartMS || details.EndMS != chunks[i].EndMS || content.Model != openrouter.TranscriptionModel || content.DurationMS != durationMS || json.Unmarshal(content.CuesJSON, &cues) != nil || content.ContentHash != podcastSHA256(content.CuesJSON) || validateTranscriptCues(cues, durationMS, true) != nil {
			return openrouter.TranscriptionResult{}, false, processingError("cached_transcript_invalid", nil)
		}
		return podcastResultFromCues(chunks[i], cues, details.Language, content.UsageJSON), true, nil
	}
	results, err := transcribePodcastAudioChunks(ctx, chunks,
		func(ctx context.Context, chunk podcastAudioChunk) error {
			_, cached, err := loadChunk(chunkByPath[chunk.Path])
			if err != nil || cached {
				return err
			}
			return convertPodcastChunk(ctx, input, chunk)
		},
		func(ctx context.Context, path, language string) (openrouter.TranscriptionResult, error) {
			i := chunkByPath[path]
			if cached, ok, err := loadChunk(i); err != nil || ok {
				return cached, err
			}
			result, err := transcribePodcastChunk(ctx, db, path, language)
			if err != nil {
				return result, err
			}
			cues, err := mergePodcastTranscription(chunks[i:i+1], []openrouter.TranscriptionResult{result}, durationMS)
			if err != nil {
				return result, err
			}
			encoded, _ := json.Marshal(cues)
			lang := normalizedTranscriptionLanguage(result.Language)
			if lang == "" {
				lang = language
			}
			details, _ := json.Marshal(podcastChunkInputs{AudioHash: audioHash, Index: i, StartMS: chunks[i].StartMS, EndMS: chunks[i].EndMS, Language: lang, SourceKey: source.transcriptKey})
			content := database.PodcastTranscriptContent{VideoID: v.ID, SourceKey: cacheKey(i), Source: "generated_chunk", SourceURL: v.MediaURL.String, ContentHash: podcastSHA256(encoded), Model: openrouter.TranscriptionModel, DurationMS: durationMS, CuesJSON: encoded, UsageJSON: result.Usage, InputJSON: details}
			if len(content.UsageJSON) == 0 {
				content.UsageJSON = []byte("{}")
			}
			if err := db.SavePodcastTranscriptContent(ctx, content); err != nil {
				return result, err
			}
			return podcastResultFromCues(chunks[i], cues, lang, result.Usage), nil
		},
		func(i int, result openrouter.TranscriptionResult) error {
			if ready == nil {
				return nil
			}
			return ready(podcastTranscriptChunk{Index: i, Chunks: chunks, Result: result, DurationMS: durationMS, Source: "generated", SourceURL: v.MediaURL.String, InputJSON: provenance})
		},
	)
	if err != nil {
		return database.PodcastTranscriptContent{}, processingError("transcription_failed", err)
	}
	metrics.ObservePodcastProcessingStage("transcription", "ready", time.Since(started).Seconds())
	cues, err := mergePodcastTranscription(chunks, results, durationMS)
	if err != nil {
		return database.PodcastTranscriptContent{}, processingError("transcription_output_invalid", err)
	}
	encoded, err := json.Marshal(cues)
	if err != nil {
		return database.PodcastTranscriptContent{}, err
	}
	usages := make([]json.RawMessage, 0, len(results))
	for _, result := range results {
		if len(result.Usage) > 0 {
			usages = append(usages, result.Usage)
		} else {
			usages = append(usages, json.RawMessage("{}"))
		}
	}
	usage, _ := json.Marshal(struct {
		AudioHash string            `json:"audio_sha256"`
		Language  string            `json:"language,omitempty"`
		Chunks    []json.RawMessage `json:"chunks"`
	}{audioHash, normalizedTranscriptionLanguage(results[0].Language), usages})
	var completeInputs map[string]any
	_ = json.Unmarshal(provenance, &completeInputs)
	completeInputs["language"] = normalizedTranscriptionLanguage(results[0].Language)
	provenance, _ = json.Marshal(completeInputs)
	return database.PodcastTranscriptContent{Source: "generated", SourceURL: v.MediaURL.String, Model: openrouter.TranscriptionModel, DurationMS: durationMS, CuesJSON: encoded, UsageJSON: usage, InputJSON: provenance}, nil
}

// Establish one language from the opening chunk before parallel uploads. Small
// outro/music chunks otherwise can switch languages and hallucinate subtitles.
func transcribePodcastAudioChunks(ctx context.Context, chunks []podcastAudioChunk,
	convert func(context.Context, podcastAudioChunk) error,
	transcribe func(context.Context, string, string) (openrouter.TranscriptionResult, error),
	ready ...func(int, openrouter.TranscriptionResult) error,
) ([]openrouter.TranscriptionResult, error) {
	if len(chunks) == 0 {
		return nil, errors.New("no audio chunks")
	}
	results := make([]openrouter.TranscriptionResult, len(chunks))
	if err := convert(ctx, chunks[0]); err != nil {
		return nil, err
	}
	first, err := transcribe(ctx, chunks[0].Path, "")
	if err != nil {
		return nil, err
	}
	results[0] = first
	for _, f := range ready {
		if err := f(0, first); err != nil {
			return nil, err
		}
	}
	language := normalizedTranscriptionLanguage(first.Language)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	group, gctx := errgroup.WithContext(cctx)
	group.SetLimit(4)
	var conversionErr error
	for i := 1; i < len(chunks); i++ {
		if err := convert(gctx, chunks[i]); err != nil {
			conversionErr = err
			cancel()
			break
		}
		index := i
		group.Go(func() error {
			result, err := transcribe(gctx, chunks[index].Path, language)
			if err != nil {
				return err
			}
			results[index] = result
			for _, f := range ready {
				if err := f(index, result); err != nil {
					return err
				}
			}
			return nil
		})
	}
	uploadErr := group.Wait()
	if conversionErr != nil {
		return nil, conversionErr
	}
	return results, uploadErr
}

func normalizedTranscriptionLanguage(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	names := map[string]string{"english": "en", "spanish": "es", "french": "fr", "german": "de", "portuguese": "pt", "russian": "ru", "italian": "it", "japanese": "ja", "chinese": "zh", "korean": "ko", "arabic": "ar", "hindi": "hi", "dutch": "nl", "polish": "pl", "ukrainian": "uk", "turkish": "tr"}
	if code := names[raw]; code != "" {
		return code
	}
	code := strings.Split(raw, "-")[0]
	if len(code) == 2 && code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z' {
		return code
	}
	return ""
}

func transcribePodcastChunk(ctx context.Context, db database.Client, path, language string) (openrouter.TranscriptionResult, error) {
	token := cuid.New()
	// Three-minute slots outlive the provider client's 90s request and its normal
	// backoff. Renew while Retry-After delays keep a logical request in flight.
	for {
		acquired, err := db.AcquirePodcastTranscriptionSlot(ctx, token, 3*time.Minute)
		if err != nil {
			return openrouter.TranscriptionResult{}, err
		}
		if acquired {
			break
		}
		if err = waitPodcastRetry(ctx, 100*time.Millisecond); err != nil {
			return openrouter.TranscriptionResult{}, err
		}
	}
	slotCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-slotCtx.Done():
				return
			case <-ticker.C:
				ok, err := db.AcquirePodcastTranscriptionSlot(slotCtx, token, 3*time.Minute)
				if err != nil || !ok {
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		<-done
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		_ = db.ReleasePodcastTranscriptionSlot(cctx, token)
	}()
	result, err := openrouter.DefaultClient.TranscribeWithLanguage(slotCtx, path, language)
	var providerError *openrouter.TranscriptionProviderError
	if errors.As(err, &providerError) {
		log.Printf("podcast transcription HTTP %d provider=%q message=%q detail=%q", providerError.StatusCode, providerError.ProviderName, providerError.Message, providerError.Detail)
	}
	if err == nil {
		var usage struct {
			Cost float64 `json:"cost"`
		}
		if json.Unmarshal(result.Usage, &usage) == nil {
			metrics.ObservePodcastProviderCost("transcription", usage.Cost)
		}
	}
	return result, err
}

// Offset and stitch complete chunk responses. Later chunks own cues whose
// midpoint is after their core boundary. Identical overlap cues are removed;
// differing boundary text is clipped to the preceding cue's end.
func mergePodcastTranscription(chunks []podcastAudioChunk, results []openrouter.TranscriptionResult, durationMS int) ([]transcriptCue, error) {
	if len(chunks) != len(results) {
		return nil, errors.New("missing chunk responses")
	}
	cues := make([]transcriptCue, 0)
	for i, result := range results {
		chunk := chunks[i]
		previousCueCount := len(cues)
		lastStart := -1
		for _, segment := range result.Segments {
			if math.IsNaN(segment.Start) || math.IsInf(segment.Start, 0) || math.IsNaN(segment.End) || math.IsInf(segment.End, 0) || segment.Start < 0 || segment.End <= segment.Start || segment.End*1000 > float64(chunk.EndMS-chunk.StartMS)+100 {
				return nil, errors.New("invalid transcription timing")
			}
			start := int(math.Round(segment.Start*1000)) + chunk.StartMS
			end := int(math.Round(segment.End*1000)) + chunk.StartMS
			if start < lastStart || start < chunk.StartMS || end > chunk.EndMS+100 {
				return nil, errors.New("out of bounds or unordered transcription timing")
			}
			lastStart = start
			end = min(end, chunk.EndMS)
			text := cleanCueText(segment.Text)
			if text == "" {
				continue
			}
			if i > 0 && (start+end)/2 < chunk.CoreStartMS {
				continue
			}
			duplicate := false
			if i > 0 && start < chunk.CoreStartMS+podcastOverlapMS {
				for j := previousCueCount - 1; j >= 0 && cues[j].EndMS > chunk.StartMS; j-- {
					if strings.EqualFold(cues[j].Text, text) && start < cues[j].EndMS && end > cues[j].StartMS {
						duplicate = true
						break
					}
				}
			}
			if duplicate {
				continue
			}
			if i > 0 && start < chunk.CoreStartMS+podcastOverlapMS && len(cues) > 0 {
				start = max(start, cues[len(cues)-1].EndMS)
			}
			if end <= start {
				continue
			}
			cues = append(cues, transcriptCue{Index: len(cues), StartMS: start, EndMS: end, Text: text})
		}
	}
	return cues, validateTranscriptCues(cues, durationMS, true)
}

func probePodcastDuration(ctx context.Context, path string) (int, error) {
	pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(pctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=duration", "-of", "json", path).Output()
	if err != nil {
		return 0, processingError("audio_probe_failed", err)
	}
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(output, &probe) != nil {
		return 0, processingError("audio_probe_failed", nil)
	}
	seconds, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || math.IsInf(seconds, 0) || math.IsNaN(seconds) || seconds <= 0 {
		return 0, processingError("audio_duration_invalid", err)
	}
	if seconds > float64(maxPodcastDurationMS)/1000 {
		return 0, processingError("episode_too_long", nil)
	}
	return int(math.Round(seconds * 1000)), nil
}

func convertPodcastChunk(ctx context.Context, input string, chunk podcastAudioChunk) error {
	started := time.Now()
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", "file,pipe", "-ss", fmt.Sprintf("%.3f", float64(chunk.StartMS)/1000), "-i", input, "-t", fmt.Sprintf("%.3f", float64(chunk.EndMS-chunk.StartMS)/1000), "-map", "0:a:0", "-vn", "-ac", "1", "-ar", "16000", "-sample_fmt", "s16", "-c:a", "flac", chunk.Path)
	err := cmd.Run()
	outcome := "ready"
	if err != nil {
		outcome = "failed"
	}
	metrics.ObservePodcastProcessingStage("ffmpeg", outcome, time.Since(started).Seconds())
	if err != nil {
		return processingError("audio_conversion_failed", err)
	}
	return nil
}

func downloadPodcastAudio(ctx context.Context, url, path string, limit int64, identities ...*podcastSourceIdentity) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept-Encoding", "identity")
		resp, err := client.Do(req)
		delay := time.Duration(1<<attempt) * time.Second
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
		} else {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if resp.ContentLength > limit {
					resp.Body.Close()
					return "", errors.New("audio exceeds download limit")
				}
				file, err := os.Create(path)
				if err != nil {
					resp.Body.Close()
					return "", err
				}
				hash := sha256.New()
				count, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, limit+1))
				closeErr := file.Close()
				resp.Body.Close()
				if count > limit {
					return "", errors.New("audio exceeds download limit")
				}
				if closeErr != nil {
					return "", closeErr
				}
				if copyErr == nil && count > 0 {
					for _, target := range identities {
						*target = podcastIdentity(resp)
						target.Length = count
					}
					return hex.EncodeToString(hash.Sum(nil)), nil
				}
				if copyErr == nil {
					return "", errors.New("empty audio")
				}
				lastErr = copyErr
			} else {
				io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				lastErr = fmt.Errorf("audio host HTTP %d", resp.StatusCode)
				if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode < 500 {
					return "", lastErr
				}
				if raw := resp.Header.Get("Retry-After"); raw != "" {
					if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
						delay = time.Duration(seconds) * time.Second
					} else if at, err := http.ParseTime(raw); err == nil {
						delay = max(0, time.Until(at))
					}
				}
			}
		}
		if attempt == 2 {
			break
		}
		if err = waitPodcastRetry(ctx, delay); err != nil {
			return "", err
		}
	}
	return "", lastErr
}
func waitPodcastRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
