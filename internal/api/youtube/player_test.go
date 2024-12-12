package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/youtube/auth"
	"github.com/cufee/feedlr-yt/tests/mock"
	"github.com/matryer/is"
	"github.com/rs/zerolog/log"
)

func testAuthClient() (*auth.Client, error) {
	client := auth.NewClient(&mock.AuthStore{})
	authed, err := client.Authenticate(context.Background(), true)
	if err != nil {
		return nil, err
	}
	<-authed

	status := client.AuthStatus()
	if status != auth.AuthStatusAuthenticated {
		return nil, errors.New("bad auth status")
	}
	return client, nil
}

func TestGetVideoPlayerDetails(t *testing.T) {
	is := is.New(t)

	authClient, err := testAuthClient()
	is.NoErr(err)

	client, err := NewClient("<none>", authClient)
	is.NoErr(err)
	{
		_, err := client.GetVideoPlayerDetails("JpW1KrK6Xjk")
		is.True(errors.Is(err, ErrLoginRequired))
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
