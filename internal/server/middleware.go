package server

import (
	"log"
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
		if c.Get("X-Forwarded-For") != "" {
			return c.Get("X-Forwarded-For")
		}
		log.Print("No X-Forwarded-For header found")
		return trace
	},
	LimitReached: func(c *fiber.Ctx) error {
		return c.Render("429", nil, "layouts/with-head")
	},
	Storage: newRedisStore(),
})
