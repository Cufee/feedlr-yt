package logic

import (
	"github.com/matryer/is"
	"testing"
)

func TestParseTimedTranscript(t *testing.T) {
	is := is.New(t)
	cues, err := parseTimedTranscript([]byte("WEBVTT\n\n1\n00:00:01.250 --> 00:00:03.500\nHello <i>world</i>\n\n00:00:04,000 --> 00:00:05,000\nSecond cue"))
	is.NoErr(err)
	is.Equal(len(cues), 2)
	is.Equal(cues[0].StartMS, 1250)
	is.Equal(cues[0].EndMS, 3500)
	is.Equal(cues[0].Text, "Hello world")
	is.Equal(cues[1].StartMS, 4000)
}

func TestParseTimedTranscriptRejectsBadRange(t *testing.T) {
	_, err := parseTimedTranscript([]byte("00:00:04.000 --> 00:00:03.000\nNope"))
	if err == nil {
		t.Fatal("expected malformed range error")
	}
}

func TestCleanCueTextRemovesMarkup(t *testing.T) {
	is := is.New(t)
	is.Equal(cleanCueText(`<p>Acme <strong>VPN</strong></p>`), "Acme VPN")
}

func TestPodcastSegmentCategories(t *testing.T) {
	for _, category := range []string{"sponsor", "selfpromo", "interaction", "preview", "intro", "filler"} {
		if !validCategory(category) {
			t.Fatalf("%q should be a valid podcast segment category", category)
		}
	}
	for _, category := range []string{"exclusive_access", "poi_highlight", "outro", "music_offtopic"} {
		if validCategory(category) {
			t.Fatalf("%q should not be a valid podcast segment category", category)
		}
	}
}
