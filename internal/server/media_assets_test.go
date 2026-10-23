package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondl/null/v9"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/gofiber/fiber/v3"
)

type mediaAssetDB struct {
	database.Client
	enclosure, artwork string
}

func (db mediaAssetDB) GetVideoByID(context.Context, string, ...database.VideoQuery) (*models.Video, error) {
	return &models.Video{ID: "pe_episode", Type: "podcast_episode", MediaURL: null.StringFrom(db.enclosure)}, nil
}
func (db mediaAssetDB) GetChannel(context.Context, string, ...database.ChannelQuery) (*models.Channel, error) {
	return &models.Channel{Thumbnail: db.artwork}, nil
}

func TestMediaAssetsPreserveImageHEADAndCacheHeaders(t *testing.T) {
	image := []byte("image fixture bytes")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(image)
	}))
	defer upstream.Close()
	m := mediaAssets{db: mediaAssetDB{artwork: upstream.URL}, http: upstream.Client()}
	app := fiber.New()
	app.Get("/thumb/channel/:id", m.channelThumbnail)
	app.Head("/thumb/channel/:id", m.channelThumbnail)
	for _, method := range []string{"GET", "HEAD"} {
		response, err := app.Test(httptest.NewRequest(method, "/thumb/channel/creator", nil))
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != 200 || response.Header.Get("Content-Type") != "image/jpeg" || response.ContentLength != int64(len(image)) {
			t.Fatalf("%s image headers: %d, %v", method, response.StatusCode, response.Header)
		}
		if method == "HEAD" && len(data) != 0 {
			t.Fatal("HEAD returned an image body")
		}
		if response.Header.Get("Cache-Control") != thumbnailCacheControl || response.Header.Get("CDN-Cache-Control") != thumbnailEdgeCacheControl || response.Header.Get("Cloudflare-CDN-Cache-Control") != thumbnailEdgeCacheControl {
			t.Fatalf("%s lost thumbnail cache headers: %v", method, response.Header)
		}
	}
}

func TestPodcastEnclosureRedirectPreservesMediaNavigation(t *testing.T) {
	m := mediaAssets{db: mediaAssetDB{enclosure: "https://audio.example.test/episode.mp3"}}
	app := fiber.New()
	app.Get("/media/stream/:id", m.stream)
	app.Head("/media/stream/:id", m.stream)
	for _, method := range []string{"GET", "HEAD"} {
		request := httptest.NewRequest(method, "/media/stream/pe_episode", nil)
		request.Header.Set("Range", "bytes=10-20")
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 302 || response.Header.Get("Location") != "https://audio.example.test/episode.mp3" || !strings.Contains(response.Header.Get("Cache-Control"), "max-age=300") {
			t.Fatalf("%s enclosure response: %d %v", method, response.StatusCode, response.Header)
		}
	}
}
