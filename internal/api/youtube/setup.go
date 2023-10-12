package youtube

import (
	"os"

	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/google"
	_ "github.com/joho/godotenv/autoload"
)

var C client.YouTube = google.NewClient(os.Getenv("YOUTUBE_API_KEY"))
var Client = C
