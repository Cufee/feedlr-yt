package youtube

import (
	"github.com/cufee/feedlr-yt/internal/utils"
)

var DefaultClient = NewClient(utils.MustGetEnv("YOUTUBE_API_KEY"))
