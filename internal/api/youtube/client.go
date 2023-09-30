package youtube

import (
	"os"

	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YouTubeClient struct {
	service *youtube.Service
}

var Client *YouTubeClient

func init() {
	opts := option.WithAPIKey(os.Getenv("YOUTUBE_API_KEY"))
	service, err := youtube.NewService(context.Background(), opts)
	if err != nil {
		panic(err)
	}

	Client = &YouTubeClient{
		service: service,
	}
}
