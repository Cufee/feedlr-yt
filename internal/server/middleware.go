package server

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
)

var limiterMiddleware = limiter.New(limiter.Config{
	Max:        100,
	Expiration: 30 * time.Second,
	KeyGenerator: func(c *fiber.Ctx) string {
		trace := c.Cookies("trace_id")
		if trace == "" {
			trace = uuid.NewString()
			cookie := fiber.Cookie{
				Name:  "trace_id",
				Value: trace,
			}
			c.Cookie(&cookie)
		}
		return c.Get("X-Forwarded-For", trace)
	},
	LimitReached: func(c *fiber.Ctx) error {
		c.Set("HX-Redirect", "/429")
		return c.Redirect("/429")
	},
})

var cacheBusterMiddleware = func(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Next()
}

var staticWithCacheMiddleware = func(path string, assets fs.FS) func(*fiber.Ctx) error {
	hashes := getAssetsHashes(assets)
	hashedAssets := make(map[string]string, len(hashes))
	for assetPath, hash := range hashes {
		hashedAssets[hashedAssetPath(path, assetPath, hash)] = assetPath
	}
	handler := filesystem.New(filesystem.Config{
		Root:       http.FS(assets),
		Browse:     true,
		PathPrefix: path,
		MaxAge:     0,
	})

	return func(c *fiber.Ctx) error {
		requestedPath := c.Path()
		assetPath, ok := hashedAssets[requestedPath]
		if ok {
			c.Path(assetPath)
			c.Set("Vary", "Accept-Encoding")
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
			return handler(c)
		}

		hash, ok := hashes[requestedPath]
		if !ok {
			return handler(c)
		}

		c.Set("Cache-Control", "no-store")
		return c.Redirect(hashedAssetPath(path, requestedPath, hash), http.StatusTemporaryRedirect)
	}
}

func hashedAssetPath(root, assetPath, hash string) string {
	return "/" + root + "/_" + "/" + hash + "/" + strings.TrimPrefix(assetPath, "/"+root+"/")
}

func getAssetsHashes(assets fs.FS) map[string]string {
	assetsHashes := make(map[string]string)

	err := fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		// Get SHA256 hash of the file
		file, err := assets.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}

		// Save hash to the map
		assetsHashes["/"+path] = fmt.Sprintf("%x", hash.Sum(nil))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return assetsHashes
}

func outageMiddleware(c *fiber.Ctx) error {
	if os.Getenv("MAINTENANCE_MODE") != "true" {
		return c.Next()
	}
	return pages.Outage().Render(c.Context(), c)
}

func requestMetricsMiddleware(c *fiber.Ctx) error {
	err := c.Next()

	route := "unknown"
	if r := c.Route(); r != nil {
		route = r.Path
	}

	status := c.Response().StatusCode()
	if status == 0 {
		if err != nil {
			status = fiber.StatusInternalServerError
		} else {
			status = fiber.StatusOK
		}
	}
	metrics.IncHTTPRequest(c.Method(), route, status)

	return err
}
