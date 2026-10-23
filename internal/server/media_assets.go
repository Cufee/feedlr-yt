package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/gofiber/fiber/v3"
)

const thumbnailCacheControl = "public, max-age=3600, s-maxage=604800, stale-while-revalidate=86400"
const thumbnailEdgeCacheControl = "public, max-age=604800, stale-while-revalidate=86400"

type mediaAssets struct {
	db   database.Client
	http *http.Client
}

func registerMediaAssets(app *fiber.App, db database.Client) {
	m := mediaAssets{db: db, http: &http.Client{Timeout: 10 * time.Second}}
	app.Get("/thumb/video/:id/:variant", m.videoThumbnail)
	app.Head("/thumb/video/:id/:variant", m.videoThumbnail)
	app.Get("/thumb/channel/:id", m.channelThumbnail)
	app.Head("/thumb/channel/:id", m.channelThumbnail)
	app.Get("/media/stream/:id", m.stream)
	app.Head("/media/stream/:id", m.stream)
}

func (m mediaAssets) image(c fiber.Ctx, imageURL string) error {
	req, err := http.NewRequestWithContext(c.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		return c.SendStatus(fiber.StatusBadGateway)
	}
	response, err := m.http.Do(req)
	if err != nil {
		return c.SendStatus(fiber.StatusBadGateway)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return c.SendStatus(fiber.StatusNotFound)
	}
	const maxImageBytes = 10 << 20
	data, err := io.ReadAll(io.LimitReader(response.Body, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		return c.SendStatus(fiber.StatusBadGateway)
	}
	contentType := strings.Split(response.Header.Get("Content-Type"), ";")[0]
	if !strings.HasPrefix(contentType, "image/") {
		return c.SendStatus(fiber.StatusBadGateway)
	}
	c.Set("Content-Type", contentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Cache-Control", thumbnailCacheControl)
	c.Set("CDN-Cache-Control", thumbnailEdgeCacheControl)
	c.Set("Cloudflare-CDN-Cache-Control", thumbnailEdgeCacheControl)
	return c.Send(data)
}

func videoThumbnailFile(variant string) (string, bool) {
	switch variant {
	case "0", "default":
		return "0.jpg", true
	case "sddefault", "hqdefault", "maxresdefault":
		return variant + ".jpg", true
	default:
		return "", false
	}
}

func (m mediaAssets) videoThumbnail(c fiber.Ctx) error {
	id := strings.TrimSpace(c.Params("id"))
	video, err := m.db.GetVideoByID(c.Context(), id, database.Video.WithChannel())
	if err != nil {
		if database.IsErrNotFound(err) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if youtube.VideoType(video.Type) == youtube.VideoTypePodcastEpisode {
		if video.R == nil || video.R.Channel == nil || video.R.Channel.Thumbnail == "" {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return m.image(c, video.R.Channel.Thumbnail)
	}
	file, ok := videoThumbnailFile(c.Params("variant"))
	if !ok {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	return m.image(c, fmt.Sprintf("https://i.ytimg.com/vi/%s/%s", id, file))
}

func (m mediaAssets) channelThumbnail(c fiber.Ctx) error {
	channel, err := m.db.GetChannel(c.Context(), c.Params("id"))
	if err != nil {
		if database.IsErrNotFound(err) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if channel.Thumbnail == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return m.image(c, channel.Thumbnail)
}

// Redirecting the enclosure keeps rotating upstream URLs out of the JSON
// contract. Browsers retain Range/HEAD semantics when following this hop.
func (m mediaAssets) stream(c fiber.Ctx) error {
	video, err := m.db.GetVideoByID(c.Context(), c.Params("id"))
	if err != nil {
		if database.IsErrNotFound(err) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if youtube.VideoType(video.Type) != youtube.VideoTypePodcastEpisode || !video.MediaURL.Valid {
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Set("Cache-Control", "public, max-age=300")
	return c.Redirect().Status(fiber.StatusFound).To(video.MediaURL.String)
}
