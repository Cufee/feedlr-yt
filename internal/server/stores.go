package server

import (
	"runtime"

	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/storage/redis/v3"
)

func newRedisStore() fiber.Storage {
	// Initialize custom config
	return redis.New(redis.Config{
		URL:      utils.MustGetEnv("REDIS_URL"),
		PoolSize: 10 * runtime.GOMAXPROCS(0),
	})
}
