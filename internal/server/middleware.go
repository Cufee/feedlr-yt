package server

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cufee/feedlr-yt/internal/templates/pages"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
)

var limiterMiddleware = limiter.New(limiter.Config{
	Max:        20,
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
	Storage: newRedisStore(),
})

var cacheBusterMiddleware = func(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Next()
}

var staticWithCacheMiddleware = func(path string, assets fs.FS) func(*fiber.Ctx) error {
	hashes := getAssetsHashes(assets)
	handler := filesystem.New(filesystem.Config{
		Root:       http.FS(assets),
		Browse:     true,
		PathPrefix: path,
		MaxAge:     86400,
	})

	return func(c *fiber.Ctx) error {
		if c.Get("If-None-Match") == hashes[c.Path()] {
			return c.SendStatus(fiber.StatusNotModified)
		}
		err := handler(c)
		if hash, ok := hashes[c.Path()]; ok {
			c.Set("Vary", "Accept-Encoding")
			c.Set("ETag", hash)
		}
		return err
	}
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
	return c.Render("layouts/HeadOnly", pages.Outage())
}
