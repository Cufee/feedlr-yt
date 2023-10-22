package utils

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
)

func MustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("missing environment variable " + key)
	}
	return value
}
