package server

import (
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/cufee/feedlr-yt/internal/utils"
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
		return c.Redirect("/429")
	},
	Storage: newRedisStore(),
})

var cacheBusterMiddleware = func(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Next()
}

var staticWithCacheMiddleware = func(path string, assets fs.FS) func(*fiber.Ctx) error {
	handler := filesystem.New(filesystem.Config{
		Root:       http.FS(assets),
		PathPrefix: path,
		Browse:     true,
		MaxAge:     86400, // 1 day
	})
	return func(c *fiber.Ctx) error {
		err := handler(c)
		if c.Path() == utils.CurrentStylePath {
			// This style is generated and will not be present in the next build - cache it forever
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		log.Print(c.Path(), err)
		return err
	}

}
