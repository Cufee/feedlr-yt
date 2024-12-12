package auth

import (
	"io"
	"net/http"
	"testing"

	"github.com/matryer/is"
)

func TestNewWebPlayerRequestContext(t *testing.T) {
	is := is.New(t)
	client := Client{http: http.DefaultClient}
	context, err := client.newWebPlayerRequestContext()
	is.NoErr(err)

	prepared, err := context.ForVideo("token", "video-1")
	is.NoErr(err)

	data, err := io.ReadAll(prepared)
	is.NoErr(err)
	println(string(data))
}
