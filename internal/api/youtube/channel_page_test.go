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

func TestChannelUploadsPlaylistID(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		want      string
		wantErr   bool
	}{
		{
			name:      "standard channel",
			channelID: "UCYO_jab_esuFRV4b17AJtAw",
			want:      "UUYO_jab_esuFRV4b17AJtAw",
		},
		{
			name:      "hyphens and underscores",
			channelID: "UCBR8-60-B28hp2BmDPdntcQ",
			want:      "UUBR8-60-B28hp2BmDPdntcQ",
		},
		{name: "wrong prefix", channelID: "UUYO_jab_esuFRV4b17AJtAw", wantErr: true},
		{name: "too short", channelID: "UC123", wantErr: true},
		{name: "too long", channelID: "UCYO_jab_esuFRV4b17AJtAwx", wantErr: true},
		{name: "invalid characters", channelID: "UCYO/jab/esuFRV4b17AJtAw", wantErr: true},
		{name: "surrounding whitespace", channelID: " UCYO_jab_esuFRV4b17AJtAw", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChannelUploadsPlaylistID(tt.channelID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ChannelUploadsPlaylistID(%q) succeeded with %q", tt.channelID, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
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
