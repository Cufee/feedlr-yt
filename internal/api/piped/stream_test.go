package piped

import (
	"context"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestClientVideo(t *testing.T) {
	is := is.New(t)

	client, err := NewClient("https://piped-api.byvko.dev")
	is.NoErr(err)

	video, err := client.Video(context.Background(), "3dKB_bM4FCU")
	is.NoErr(err)
	is.True(video.UploadTimestamp > 0)
	is.True(video.PublishDate().Year() > time.Time{}.AddDate(10, 0, 0).Year())
}
