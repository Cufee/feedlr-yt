package rss

import (
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestParseDuration(t *testing.T) {
	is := is.New(t)

	cases := map[string]int{
		"5035":       5035,
		" 434 ":      434,
		"0":          0,
		"56:37":      3397,
		"1:23:45":    5025,
		"1:02:03:04": 93784,
		"":           0,
		"abc":        0,
		"-5":         0,
		"1:xx":       0,
		"1:2:3:4:5":  0,
	}
	for raw, expected := range cases {
		is.Equal(ParseDuration(raw), expected) // raw: +raw
	}
}

func TestShowIDIsDeterministicAndURLSafe(t *testing.T) {
	is := is.New(t)

	id := ShowID("https://thestanduppod.com/feed.xml")
	is.True(strings.HasPrefix(id, "pc_"))
	is.True(len(id) == 23)
	is.Equal(id, ShowID("https://thestanduppod.com/feed.xml"))
	is.True(id != ShowID("https://other.example.com/feed.xml"))
	is.True(strings.Trim(id[3:], "0123456789abcdef") == "")
}

func TestEpisodeIDIsDeterministicAndScopedByFeed(t *testing.T) {
	is := is.New(t)

	guid := "flightcast:01M0KE9YFD1ECBXW0DTVJT017R"
	id := EpisodeID("https://feed.example.com/rss", guid)
	is.True(strings.HasPrefix(id, "pe_"))
	is.Equal(id, EpisodeID("https://feed.example.com/rss", guid))
	// same GUID on a different feed must map to a different episode
	is.True(id != EpisodeID("https://other.example.com/rss", guid))
}

func TestParseFeedExtractsShowAndEpisodes(t *testing.T) {
	is := is.New(t)

	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>The Standup</title>
    <description>Software, life, memes and more.</description>
    <itunes:image href="https://img.example.com/cover.jpg"/>
    <itunes:author>ThePrimeagen</itunes:author>
    <item>
      <title>Episode B</title>
      <guid isPermaLink="false">flightcast:episode-b</guid>
      <pubDate>Mon, 10 Aug 2026 13:00:00 +0000</pubDate>
      <description>Second episode</description>
      <enclosure url="https://episode.example.com/b.mp3" length="0" type="audio/mpeg"/>
      <itunes:duration>1:23:45</itunes:duration>
    </item>
    <item>
      <title>Episode A</title>
      <guid isPermaLink="false">flightcast:episode-a</guid>
      <pubDate>Tue, 18 Aug 2026 15:00:00 +0000</pubDate>
      <description>First episode</description>
      <enclosure url="https://episode.example.com/a.mp3" length="0" type="audio/mpeg"/>
      <itunes:duration>434</itunes:duration>
    </item>
    <item>
      <title>Episode without enclosure</title>
      <guid isPermaLink="false">flightcast:no-media</guid>
      <pubDate>Wed, 19 Aug 2026 15:00:00 +0000</pubDate>
      <description>Should be skipped</description>
    </item>
    <item>
      <title>Episode without date</title>
      <guid isPermaLink="false">flightcast:no-date</guid>
      <enclosure url="https://episode.example.com/nodate.mp3" length="0" type="audio/mpeg"/>
      <itunes:duration>120</itunes:duration>
    </item>
  </channel>
</rss>`

	result, err := parseFeed(strings.NewReader(feed))
	is.NoErr(err)

	is.Equal(result.Show.Title, "The Standup")
	is.Equal(result.Show.ImageURL, "https://img.example.com/cover.jpg")
	is.Equal(result.Show.Author, "ThePrimeagen")

	is.Equal(len(result.Episodes), 2)

	// newest first
	is.Equal(result.Episodes[0].Title, "Episode A")
	is.Equal(result.Episodes[0].GUID, "flightcast:episode-a")
	is.Equal(result.Episodes[0].Duration, 434)
	is.Equal(result.Episodes[0].MediaURL, "https://episode.example.com/a.mp3")
	is.Equal(result.Episodes[0].PublishedAt.UTC(), time.Date(2026, 8, 18, 15, 0, 0, 0, time.UTC))

	is.Equal(result.Episodes[1].Title, "Episode B")
	is.Equal(result.Episodes[1].Duration, 5025)
}

func TestEpisodeIDMatchesEpisodeUpsert(t *testing.T) {
	is := is.New(t)

	// Deterministic IDs are the dedup mechanism for episode upserts: the same
	// feed item must always map to the same row across polls.
	id := EpisodeID("https://feed.example.com/rss", "flightcast:episode-a")
	is.True(id != EpisodeID("https://feed.example.com/rss", "flightcast:episode-b"))
}
