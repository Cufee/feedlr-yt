package server

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/gofiber/fiber/v2"
)

type chapterCacheEntry struct {
	timeline youtube.VideoTimeline
	expires  time.Time
}

type chapterFlight struct {
	done     chan struct{}
	timeline youtube.VideoTimeline
}

// The cache bounds both stored metadata and simultaneous upstream requests.
// A short failure TTL lets temporary blocks recover without retry storms.
type chapterCache struct {
	mu      sync.Mutex
	entries map[string]chapterCacheEntry
	flights map[string]*chapterFlight
	now     func() time.Time
	load    func(context.Context, string, float64, string) (youtube.VideoTimeline, error)
}

func newChapterCache() *chapterCache {
	return &chapterCache{
		entries: make(map[string]chapterCacheEntry), flights: make(map[string]*chapterFlight), now: time.Now,
		load: func(ctx context.Context, id string, duration float64, description string) (youtube.VideoTimeline, error) {
			if youtube.DefaultClient == nil {
				return youtube.VideoTimeline{}, errors.New("YouTube client unavailable")
			}
			return youtube.DefaultClient.GetVideoTimeline(ctx, id, duration)
		},
	}
}

func (cache *chapterCache) get(ctx context.Context, id string, duration float64, description string) youtube.VideoTimeline {
	cache.mu.Lock()
	if entry, ok := cache.entries[id]; ok && entry.expires.After(cache.now()) {
		cache.mu.Unlock()
		return entry.timeline
	}
	flight, ok := cache.flights[id]
	if !ok {
		if len(cache.flights) >= 8 {
			cache.mu.Unlock()
			return youtube.VideoTimeline{Chapters: youtube.ChaptersFromDescription(description, duration)}
		}
		flight = &chapterFlight{done: make(chan struct{})}
		cache.flights[id] = flight
		go cache.fetch(id, duration, description, flight)
	}
	cache.mu.Unlock()
	select {
	case <-ctx.Done():
		return youtube.VideoTimeline{Chapters: youtube.ChaptersFromDescription(description, duration)}
	case <-flight.done:
		return flight.timeline
	}
}

func (cache *chapterCache) fetch(id string, duration float64, description string, flight *chapterFlight) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	timeline, err := cache.load(ctx, id, duration, description)
	if len(timeline.Chapters) == 0 {
		timeline.Chapters = youtube.ChaptersFromDescription(description, duration)
	}
	ttl := 6 * time.Hour
	if err != nil {
		ttl = time.Minute
	} else if len(timeline.Chapters) == 0 && timeline.Storyboard == nil {
		ttl = time.Hour
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) >= 256 {
		var oldestID string
		var oldest time.Time
		for key, entry := range cache.entries {
			if oldestID == "" || entry.expires.Before(oldest) {
				oldestID, oldest = key, entry.expires
			}
		}
		delete(cache.entries, oldestID)
	}
	cache.entries[id] = chapterCacheEntry{timeline: timeline, expires: cache.now().Add(ttl)}
	flight.timeline = timeline
	delete(cache.flights, id)
	close(flight.done)
}

func (cache *chapterCache) peek(id string) youtube.VideoTimeline {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry := cache.entries[id]
	if !entry.expires.After(cache.now()) {
		return youtube.VideoTimeline{}
	}
	return entry.timeline
}

type chapterThumbnailClient interface {
	GetChapterThumbnail(context.Context, string) ([]byte, string, error)
}

type chapterRoutes struct {
	db              playbackDatabase
	cache           *chapterCache
	thumbnailClient chapterThumbnailClient
}

func (r chapterRoutes) chapters(c *fiber.Ctx) error {
	id := strings.Clone(c.Params("id"))
	if !playbackVideoID.MatchString(id) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	video, err := r.db.GetVideoByID(ctx, id)
	if err != nil || video == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "video_unavailable"})
	}
	if youtube.VideoType(video.Type) == youtube.VideoTypePodcastEpisode {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	timeline := r.cache.get(ctx, id, float64(video.Duration), video.Description)
	if timeline.Chapters == nil {
		timeline.Chapters = []youtube.Chapter{}
	}
	return c.JSON(timeline)
}

func (r chapterRoutes) thumbnail(c *fiber.Ctx) error {
	id := strings.Clone(c.Params("id"))
	index, err := strconv.Atoi(c.Params("index"))
	if !playbackVideoID.MatchString(id) || err != nil || index < 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	chapters := r.cache.peek(id).Chapters
	if index >= len(chapters) {
		return c.SendStatus(fiber.StatusNotFound)
	}
	var image youtube.ChapterThumbnail
	for _, candidate := range chapters[index].Thumbnails {
		if candidate.Width > image.Width {
			image = candidate
		}
	}
	if image.URL == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return r.sendThumbnail(c, image.URL)
}

func (r chapterRoutes) storyboardThumbnail(c *fiber.Ctx) error {
	id := strings.Clone(c.Params("id"))
	index, err := strconv.Atoi(c.Params("index"))
	if !playbackVideoID.MatchString(id) || err != nil || index < 0 {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	storyboard := r.cache.peek(id).Storyboard
	if storyboard == nil || index >= len(storyboard.Sheets) || storyboard.Sheets[index] == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return r.sendThumbnail(c, storyboard.Sheets[index])
}

func (r chapterRoutes) sendThumbnail(c *fiber.Ctx, imageURL string) error {
	client := r.thumbnailClient
	if client == nil {
		if youtube.DefaultClient == nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		client = youtube.DefaultClient
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	body, contentType, err := client.GetChapterThumbnail(ctx, imageURL)
	if err != nil {
		return c.SendStatus(fiber.StatusBadGateway)
	}
	c.Set("Content-Type", contentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", "private, max-age=3600")
	return c.Send(body)
}
