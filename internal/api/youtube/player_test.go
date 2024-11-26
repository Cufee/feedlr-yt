package youtube

import (
	"encoding/json"
	"testing"

	"github.com/matryer/is"
	"github.com/rs/zerolog/log"
)

func TestGetVideoPlayerDetails(t *testing.T) {
	is := is.New(t)

	authClient, err := testAuthClient()
	is.NoErr(err)

	client, err := NewClient("<none>", authClient)
	is.NoErr(err)
	{
		video, err := client.GetVideoPlayerDetails("JpW1KrK6Xjk")
		is.NoErr(err)

		e, err := json.MarshalIndent(video, "", "  ")
		is.NoErr(err)

		is.True(video.Type == VideoTypePrivate)
		log.Print(string(e))
	}
	{
		video, err := client.GetVideoPlayerDetails("LaRKIwpGPTU")
		is.NoErr(err)

		e, err := json.MarshalIndent(video, "", "  ")
		is.NoErr(err)

		is.True(video.Type == VideoTypeVideo)
		is.True(video.Duration > 200)
		log.Print(string(e))
	}
	{
		video, err := client.GetVideoPlayerDetails("OSd9935ltj8")
		is.NoErr(err)

		e, err := json.MarshalIndent(video, "", "  ")
		is.NoErr(err)

		is.True(video.Type == VideoTypeShort)
		log.Print(string(e))
	}
}
