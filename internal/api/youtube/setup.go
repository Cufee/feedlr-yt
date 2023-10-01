package youtube

import (
	"os"

	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/google"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/invidious"
	_ "github.com/joho/godotenv/autoload"
)

var C client.YouTube
var Client = C

func InitGoogleClient() {
	C = google.NewClient(os.Getenv("YOUTUBE_API_KEY"))
}

func InitInvidiousClient() {
	C = invidious.NewClient(os.Getenv("INVIDIOUS_HOST"))
}

func init() {
	flavor := os.Getenv("YOUTUBE_API_FLAVOR")
	switch flavor {
	case "google":
		InitGoogleClient()
	case "invidious":
		InitInvidiousClient()
	default:
		InitGoogleClient()
	}
}
