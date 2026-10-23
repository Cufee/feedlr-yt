package server

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/cufee/feedlr-yt/internal/metrics"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

var cacheBusterMiddleware = func(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Next()
}

var staticWithCacheMiddleware = func(path string, assets fs.FS) func(fiber.Ctx) error {
	// Development passes os.DirFS("."). Restrict both startup hashing and file
	// serving to the asset directory before walking the filesystem.
	scopedAssets, err := fs.Sub(assets, path)
	if err != nil {
		panic(fmt.Errorf("scope static assets: %w", err))
	}
	relativeHashes := getAssetsHashes(scopedAssets)
	hashes := make(map[string]string, len(relativeHashes))
	for assetPath, hash := range relativeHashes {
		hashes["/"+path+assetPath] = hash
	}
	hashedAssets := make(map[string]string, len(hashes))
	for assetPath, hash := range hashes {
		hashedAssets[hashedAssetPath(path, assetPath, hash)] = assetPath
	}
	handler := static.New(".", static.Config{
		FS:     scopedAssets,
		Browse: false,
		MaxAge: 0,
	})

	return func(c fiber.Ctx) error {
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
		return c.Redirect().Status(http.StatusTemporaryRedirect).To(hashedAssetPath(path, requestedPath, hash))
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

func requestMetricsMiddleware(c fiber.Ctx) error {
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
