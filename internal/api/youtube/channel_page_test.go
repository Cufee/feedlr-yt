package youtube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const testPageChannelID = "UCYO_jab_esuFRV4b17AJtAw"

type channelPageTransport func(*http.Request) (*http.Response, error)

func (f channelPageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetChannelPageUsesPublicPage(t *testing.T) {
	c := &client{http: &http.Client{Transport: channelPageTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Host != "www.youtube.com" || r.URL.Path != "/channel/"+testPageChannelID || r.URL.Query().Get("key") != "" || r.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected channel request: %s %s", r.Method, r.URL)
		}
		body := `<script>var ytInitialData = {"metadata":{"channelMetadataRenderer":{"externalId":"` + testPageChannelID + `","title":"A & B","description":"Line one\nLine two","avatar":{"thumbnails":[{"url":"//example.com/avatar"}]}}}}; anotherFunction();</script>`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	channel, err := c.GetChannelPage(context.Background(), testPageChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if channel.Title != "A & B" || channel.Description != "Line one\nLine two" || channel.Thumbnail != "https://example.com/avatar" {
		t.Fatalf("unexpected metadata: %+v", channel)
	}
}

func TestParseChannelPageRejectsUnavailableAndMismatchedPages(t *testing.T) {
	for _, body := range []string{
		`<html>Before you continue to YouTube</html>`,
		`<script>var ytInitialData = {broken JSON};</script>`,
		`<script>var ytInitialData = {"metadata":{"channelMetadataRenderer":{"externalId":"another-channel","title":"Wrong channel"}}};</script>`,
	} {
		if _, err := parseChannelPage(strings.NewReader(body), testPageChannelID); err == nil {
			t.Fatalf("accepted invalid page: %s", body)
		}
	}
	body := `<script>window["ytInitialData"] = {"metadata":{"channelMetadataRenderer":{"externalId":"` + testPageChannelID + `","title":"Channel without description"}}};</script>`
	if _, err := parseChannelPage(strings.NewReader(body), testPageChannelID); err != nil {
		t.Fatal(err)
	}
}

func TestGetChannelPageErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusTooManyRequests} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := &client{http: &http.Client{Transport: channelPageTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
			})}}
			if _, err := c.GetChannelPage(context.Background(), testPageChannelID); err == nil {
				t.Fatal("expected HTTP error")
			}
		})
	}
	c := &client{http: &http.Client{Transport: channelPageTransport(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetChannelPage(ctx, testPageChannelID); err == nil {
		t.Fatal("expected cancellation")
	}
	if _, err := c.GetChannelPage(context.Background(), "../../bad"); err == nil {
		t.Fatal("expected invalid ID error")
	}
}
