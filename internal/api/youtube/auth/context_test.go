package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestNewWebPlayerRequestContext(t *testing.T) {
	if os.Getenv("FEEDLR_INTEGRATION_TESTS") == "" {
		t.Skip("set FEEDLR_INTEGRATION_TESTS=1 to run integration tests against live services")
	}

	is := is.New(t)
	client := Client{http: http.DefaultClient}
	context, err := client.newWebPlayerRequestContext(context.Background())
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

func TestWebPlayerContextFetchHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	// Redirect only this test client's transport, without changing the shared
	// YouTube URL or making any external requests.
	client := Client{http: &http.Client{Transport: contextTestTransport{server.Client().Transport, server.URL}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.newWebPlayerRequestContext(ctx)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("context fetch did not cancel")
	}
}

type contextTestTransport struct {
	base http.RoundTripper
	url  string
}

func (t contextTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	redirected, err := http.NewRequestWithContext(req.Context(), req.Method, t.url, req.Body)
	if err != nil {
		return nil, err
	}
	return t.base.RoundTrip(redirected)
}
