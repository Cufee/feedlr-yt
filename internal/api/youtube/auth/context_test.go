package auth

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestNewWebPlayerRequestContext(t *testing.T) {
	if os.Getenv("FEEDLR_INTEGRATION_TESTS") == "" {
		t.Skip("set FEEDLR_INTEGRATION_TESTS=1 to run integration tests against live services")
	}

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

func TestUserAgentsAreValid(t *testing.T) {
	const minChromeVersion = 140

	for _, ua := range userAgents {
		t.Run(ua, func(t *testing.T) {
			if !strings.Contains(ua, "AppleWebKit/537.36") {
				t.Fatal("missing AppleWebKit token")
			}

			version := extractChromeVersion(ua)
			major, _, ok := strings.Cut(version, ".")
			if !ok {
				t.Fatalf("bad version format: %s", version)
			}
			v, err := strconv.Atoi(major)
			if err != nil {
				t.Fatalf("non-numeric major version: %s", major)
			}
			if v < minChromeVersion {
				t.Fatalf("Chrome version %d is below minimum %d — update userAgents", v, minChromeVersion)
			}
		})
	}
}
