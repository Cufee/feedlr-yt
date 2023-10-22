package youtube

import (
	"github.com/byvko-dev/youtube-app/internal/api/youtube/client"
	"github.com/byvko-dev/youtube-app/internal/api/youtube/internal/google"
	"github.com/byvko-dev/youtube-app/internal/utils"
)

var C client.YouTube = google.NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
var Client = C
