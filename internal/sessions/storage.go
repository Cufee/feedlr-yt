package sessions

import (
	"log"
	"os"

	"github.com/gofiber/storage/redis/v3"
)

var Storage *redis.Storage

func init() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		log.Fatal("REDIS_URL is not set")
	}
	Storage = redis.New(redis.Config{URL: url})
}
