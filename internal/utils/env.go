package utils

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
)

func MustGetEnv(key string) string {
	if os.Getenv("BUILD_MODE") == "true" {
		return ""
	}

	value := os.Getenv(key)
	if value == "" {
		panic("missing environment variable " + key)
	}
	return value
}
