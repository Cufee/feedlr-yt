package server

import (
	"time"

	"github.com/gofiber/fiber/v2"
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

var cacheHeaderMiddleware = func(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	return c.Next()
}
