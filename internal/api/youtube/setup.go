package youtube

import (
	"github.com/cufee/feedlr-yt/internal/api/youtube/client"
	"github.com/cufee/feedlr-yt/internal/api/youtube/internal/google"
	"github.com/cufee/feedlr-yt/internal/utils"
)

var C client.YouTube = google.NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
var Client = C
