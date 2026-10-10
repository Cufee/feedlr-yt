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
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/metrics"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const podcastValidationTTL = time.Minute
const podcastValidationTimeout = 2 * time.Second
const podcastSampleBytes int64 = 32 << 10
const podcastInputVersion = "podcast-inputs-v1"

// These describe the exact backend rendition. They cannot establish that a
// browser at another address received the same dynamically inserted ads.
type podcastSourceIdentity struct {
	URL          string `json:"resolved_url"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	Length       int64  `json:"bytes,omitempty"`
	SampleHash   string `json:"sample_sha256,omitempty"`
	ContentHash  string `json:"content_sha256,omitempty"`
	Unavailable  bool   `json:"unavailable,omitempty"`
}

type podcastSourceInputs struct {
	Version   string                `json:"version"`
	Audio     podcastSourceIdentity `json:"audio"`
	Publisher podcastSourceIdentity `json:"publisher"`
}

func podcastSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Strip only known authorization/expiration parameters. Other parameters may
// identify an audio variant and must remain part of its identity.
func stablePodcastSourceURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for key := range q {
		switch strings.ToLower(key) {
		case "exp", "expires", "le", "signature", "policy", "key-pair-id", "x-amz-algorithm", "x-amz-credential", "x-amz-date", "x-amz-expires", "x-amz-signedheaders", "x-amz-signature", "x-amz-security-token":
			q.Del(key)
		}
	}
	u.RawQuery, u.Fragment = q.Encode(), ""
	return u.String()
}

func podcastIdentity(resp *http.Response) podcastSourceIdentity {
	return podcastSourceIdentity{URL: stablePodcastSourceURL(resp.Request.URL.String()), ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified"), Length: resp.ContentLength}
}

func podcastSampleOffsets(length int64) []int64 {
	if length <= podcastSampleBytes {
		return []int64{0}
	}
	last := length - podcastSampleBytes
	return []int64{0, last / 4, last / 2, last * 3 / 4, last}
}

func hashPodcastSamples(length int64, samples [][]byte) string {
	h := sha256.New()
	fmt.Fprintf(h, "podcast-audio-samples-v1:%d", length)
	for i, offset := range podcastSampleOffsets(length) {
		fmt.Fprintf(h, ":%d:%d:", offset, len(samples[i]))
		h.Write(samples[i])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashPodcastFileSamples(path string, length int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	offsets := podcastSampleOffsets(length)
	samples := make([][]byte, len(offsets))
	for i, offset := range offsets {
		samples[i] = make([]byte, min(podcastSampleBytes, length-offset))
		if _, err := f.ReadAt(samples[i], offset); err != nil {
			return "", err
		}
	}
	return hashPodcastSamples(length, samples), nil
}

// At most 160 KiB of audio is read. Use the resolved rendition URL and If-Match
// so a single check never combines ranges from different strong-ETag objects.
func probePodcastAudio(ctx context.Context, raw string) (podcastSourceIdentity, error) {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return podcastSourceIdentity{}, err
	}
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := client.Do(req)
	if err != nil {
		return podcastSourceIdentity{}, err
	}
	resp.Body.Close()
	// Some enclosures support GET ranges but reject HEAD or omit its length.
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented || (resp.StatusCode >= 200 && resp.StatusCode < 300 && resp.ContentLength <= 0) {
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return podcastSourceIdentity{}, err
		}
		req.Header.Set("Range", "bytes=0-0")
		req.Header.Set("Accept-Encoding", "identity")
		resp, err = client.Do(req)
		if err != nil {
			return podcastSourceIdentity{}, err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusPartialContent {
			prefix, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/")
			if !ok || prefix != "bytes 0-0" {
				return podcastSourceIdentity{}, errors.New("invalid audio length probe")
			}
			resp.ContentLength, err = strconv.ParseInt(total, 10, 64)
			if err != nil {
				return podcastSourceIdentity{}, err
			}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return podcastSourceIdentity{}, fmt.Errorf("audio validation HTTP %d", resp.StatusCode)
	}
	identity := podcastIdentity(resp)
	if identity.Length <= 0 || identity.Length > maxPodcastDownloadBytes {
		return identity, errors.New("audio validation has invalid or oversized length")
	}
	offsets := podcastSampleOffsets(identity.Length)
	samples := make([][]byte, len(offsets))
	var unsupportedRanges atomic.Bool
	group, gctx := errgroup.WithContext(ctx)
	group.SetLimit(4)
	for i, offset := range offsets {
		i, offset := i, offset
		group.Go(func() error {
			size := min(podcastSampleBytes, identity.Length-offset)
			req, err := http.NewRequestWithContext(gctx, http.MethodGet, resp.Request.URL.String(), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+size-1))
			req.Header.Set("Accept-Encoding", "identity")
			if identity.ETag != "" && !strings.HasPrefix(identity.ETag, "W/") {
				req.Header.Set("If-Match", identity.ETag)
			}
			r, err := client.Do(req)
			if err != nil {
				return err
			}
			defer r.Body.Close()
			if r.StatusCode == http.StatusOK && identity.Length > podcastSampleBytes {
				// Compare the complete response's identity before trusting a
				// HEAD validator on a host that ignores Range/If-Match.
				actual := podcastIdentity(r)
				if identity.ETag == "" || strings.HasPrefix(identity.ETag, "W/") || identity.ETag != actual.ETag || identity.URL != actual.URL ||
					(actual.Length > 0 && actual.Length != identity.Length) || (identity.LastModified != "" && actual.LastModified != "" && identity.LastModified != actual.LastModified) {
					return errors.New("audio host lacks usable ranges or consistent strong validators")
				}
				unsupportedRanges.Store(true)
				return nil
			}
			if r.StatusCode != http.StatusPartialContent && !(r.StatusCode == http.StatusOK && offset == 0 && identity.Length <= podcastSampleBytes) {
				return fmt.Errorf("audio sample HTTP %d", r.StatusCode)
			}
			if r.StatusCode == http.StatusPartialContent && r.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", offset, offset+size-1, identity.Length) {
				return errors.New("audio sample range changed")
			}
			if identity.ETag != "" && r.Header.Get("ETag") != "" && identity.ETag != r.Header.Get("ETag") {
				return errors.New("audio sample ETag changed")
			}
			data, err := io.ReadAll(io.LimitReader(r.Body, size+1))
			if err != nil || int64(len(data)) != size {
				return errors.New("audio sample is incomplete")
			}
			samples[i] = data
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return identity, err
	}
	if unsupportedRanges.Load() {
		return identity, nil
	}
	identity.SampleHash = hashPodcastSamples(identity.Length, samples)
	return identity, nil
}

var podcastValidations singleflight.Group

// Playback POST calls this; GET and page rendering only read its cached row.
func revalidatePodcastInput(ctx context.Context, db database.Client, input podcastInput, force bool) (podcastInput, error) {
	if !force && !input.validation.ValidatedAt.IsZero() && time.Since(input.validation.ValidatedAt) < podcastValidationTTL {
		return input, nil
	}
	key := fmt.Sprintf("%p:%s:%s", db, input.video.ID, input.metadataKey)
	result := podcastValidations.DoChan(key, func() (any, error) {
		started := time.Now()
		outcome := "failed"
		defer func() { metrics.ObservePodcastProcessingStage("validation", outcome, time.Since(started).Seconds()) }()
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), podcastValidationTimeout)
		defer cancel()
		cached, err := loadPodcastInput(cctx, db, input.video.ID)
		if err != nil {
			return nil, err
		}
		if !force && !cached.validation.ValidatedAt.IsZero() && time.Since(cached.validation.ValidatedAt) < podcastValidationTTL {
			outcome = "cached"
			return cached, nil
		}
		sources := podcastSourceInputs{Version: podcastInputVersion}
		group, gctx := errgroup.WithContext(cctx)
		group.Go(func() error {
			var err error
			sources.Audio, err = probePodcastAudio(gctx, input.video.MediaURL.String)
			return err
		})
		if input.publisher.URL != "" && isTimedTranscript(input.publisher.MIMEType) {
			group.Go(func() error {
				// Timed text is usually small. Hash its complete bytes so an
				// unchanged URL/ETag cannot conceal changed transcript content.
				data, failure := fetchTranscript(gctx, input.publisher.URL, input.publisher.MIMEType)
				sources.Publisher = podcastSourceIdentity{URL: stablePodcastSourceURL(input.publisher.URL), Unavailable: failure != ""}
				if failure == "" {
					sources.Publisher.ContentHash = podcastSHA256(data)
				}
				return nil
			})
		}
		if err := group.Wait(); err != nil {
			return nil, processingError("source_validation_failed", err)
		}
		encoded, _ := json.Marshal(sources)
		validation := database.PodcastSourceValidation{VideoID: input.video.ID, MetadataKey: input.metadataKey, Fingerprint: podcastSHA256(encoded), InputJSON: encoded, ValidatedAt: time.Now().UTC()}
		if err := db.SavePodcastSourceValidation(cctx, validation); err != nil {
			return nil, err
		}
		outcome = "ready"
		return loadPodcastInput(cctx, db, input.video.ID)
	})
	select {
	case <-ctx.Done():
		return podcastInput{}, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return podcastInput{}, result.Err
		}
		return result.Val.(podcastInput), nil
	}
}

func podcastValidationInputs(input podcastInput) podcastSourceInputs {
	var sources podcastSourceInputs
	_ = json.Unmarshal(input.validation.InputJSON, &sources)
	return sources
}

func podcastRenditionMatches(expected, actual podcastSourceIdentity) bool {
	return (expected.URL == "" || expected.URL == actual.URL) &&
		(expected.ETag == "" || expected.ETag == actual.ETag) &&
		(expected.LastModified == "" || expected.LastModified == actual.LastModified) &&
		(expected.Length <= 0 || expected.Length == actual.Length) &&
		(expected.SampleHash == "" || expected.SampleHash == actual.SampleHash)
}

func loadPodcastValidation(ctx context.Context, db database.Client, videoID, metadataKey string) (database.PodcastSourceValidation, error) {
	validation, err := db.GetPodcastSourceValidation(ctx, videoID, metadataKey)
	if errors.Is(err, sql.ErrNoRows) {
		return database.PodcastSourceValidation{}, nil
	}
	return validation, err
}
