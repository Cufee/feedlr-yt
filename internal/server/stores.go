package server

import (
	"os"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/storage/redis/v3"
)

func newRedisStore() fiber.Storage {
	// Initialize custom config
	return redis.New(redis.Config{
		URL:      os.Getenv("REDIS_URL"),
		PoolSize: 10 * runtime.GOMAXPROCS(0),
	})
}
