package webapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cufee/feedlr-yt/internal/api/podcastindex"
	"github.com/cufee/feedlr-yt/internal/api/rss"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v3"
)

func TestPodcastDiscoveryRemembersCanonicalLinksAndSubscriptionAcrossAPIInstances(t *testing.T) {
	f := newFixture(t)
	previousCatalog, previousTransport := podcastindex.DefaultClient, http.DefaultTransport
	t.Cleanup(func() { podcastindex.DefaultClient, http.DefaultTransport = previousCatalog, previousTransport })
	const catalogFeed = "https://example.test/new.xml"
	const canonicalFeed = "https://example.test/canonical.xml"
	var feedRequests atomic.Int32
	http.DefaultTransport = testTransport(func(r *http.Request) (*http.Response, error) {
		response := &http.Response{StatusCode: 200, Header: make(http.Header), Request: r}
		var body string
		switch {
		case r.URL.Host == "api.podcastindex.org":
			body = `{"status":"true","feeds":[{"id":1,"title":"Existing show","url":"https://example.test/feed.xml"},{"id":2,"title":"Fresh show","url":"` + catalogFeed + `"}]}`
		case r.URL.String() == catalogFeed:
			feedRequests.Add(1)
			response.StatusCode = 302
			response.Header.Set("Location", canonicalFeed)
		case r.URL.String() == canonicalFeed:
			feedRequests.Add(1)
			body = `<?xml version="1.0"?><rss version="2.0"><channel><title>Fresh show</title><description>New feed</description><link>https://example.test/</link><item><guid>new-episode</guid><title>New episode</title><pubDate>Wed, 21 Oct 2026 10:00:00 GMT</pubDate><enclosure url="https://example.test/new.mp3" type="audio/mpeg" length="2000"/></item></channel></rss>`
		default:
			return nil, fmt.Errorf("unexpected network request %s", r.URL)
		}
		response.Body = io.NopCloser(strings.NewReader(body))
		return response, nil
	})
	var err error
	podcastindex.DefaultClient, err = podcastindex.NewClient("test", "test")
	if err != nil {
		t.Fatal(err)
	}
	checkResults := func(wantSubscribed bool, wantID string) {
		t.Helper()
		res, value := f.request(t, "GET", "/search?source=podcasts&q=shows", "", f.session, "")
		if res.StatusCode != 200 {
			t.Fatalf("search: %d %#v", res.StatusCode, value)
		}
		f.check(t, value, "ChannelList")
		items := value.(map[string]any)["items"].([]any)
		existing, fresh := items[0].(map[string]any), items[1].(map[string]any)
		if existing["subscribed"] != true || existing["id"] != "show" {
			t.Fatalf("cached subscription forgotten: %#v", existing)
		}
		if fresh["subscribed"] != wantSubscribed || fresh["id"] != wantID {
			t.Fatalf("fresh discovery state: %#v", fresh)
		}
	}
	checkResults(false, rss.ShowID(catalogFeed))
	res, value := f.request(t, "POST", "/podcasts/open", `{"feedUrl":"https://example.test/feed.xml"}`, f.session, "")
	if res.StatusCode != 200 || value.(map[string]any)["channelId"] != "show" || feedRequests.Load() != 0 {
		t.Fatalf("existing show should open from cache: %d %#v (%d fetches)", res.StatusCode, value, feedRequests.Load())
	}
	res, value = f.request(t, "POST", "/podcasts/open", `{"feedUrl":"`+catalogFeed+`"}`, f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("open: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "PodcastOpenResult")
	canonicalID := rss.ShowID(canonicalFeed)
	if value.(map[string]any)["channelId"] != canonicalID {
		t.Fatalf("did not follow canonical feed: %#v", value)
	}
	res, value = f.request(t, "GET", "/channels/"+canonicalID, "", f.session, "")
	if res.StatusCode != 200 {
		t.Fatalf("fresh title link cannot open: %d %#v", res.StatusCode, value)
	}
	f.check(t, value, "ChannelDetail")
	if len(value.(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("feed episodes not cached: %#v", value)
	}
	checkResults(false, canonicalID)
	res, value = f.request(t, "POST", "/subscriptions", `{"feedUrl":"`+catalogFeed+`"}`, f.session, "")
	if res.StatusCode != 204 {
		t.Fatalf("subscribe: %d %#v", res.StatusCode, value)
	}
	// A fresh API object must recover aliases and subscriptions from SQLite.
	sc, err := sessions.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	f.app = fiber.New()
	Register(f.app, f.db, sc, nil)
	checkResults(true, canonicalID)
	before := feedRequests.Load()
	res, value = f.request(t, "POST", "/podcasts/open", `{"feedUrl":"`+catalogFeed+`"}`, f.session, "")
	if res.StatusCode != 200 || value.(map[string]any)["channelId"] != canonicalID || feedRequests.Load() != before {
		t.Fatalf("persisted catalog alias lost: %d %#v", res.StatusCode, value)
	}
	aliases := f.db.(interface {
		GetPodcastFeedAlias(context.Context, string) (string, error)
	})
	if id, err := aliases.GetPodcastFeedAlias(context.Background(), "HTTPS://EXAMPLE.TEST/new.xml#ignored"); err != nil || id != canonicalID {
		t.Fatalf("normalized alias unavailable: %s %v", id, err)
	}
}
