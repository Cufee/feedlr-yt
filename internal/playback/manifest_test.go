package playback

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManifestPreservesAudioTrackMetadataAndResourceMapping(t *testing.T) {
	f := newFake(t)
	s := newService(t, f)
	expiry := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	// A localized dub can precede the original, and language alone cannot
	// distinguish the original from another dub or a descriptive track.
	const input = `<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT60S"><Period>
  <AdaptationSet id="en-dub" mimeType="audio/mp4" contentType="audio" lang="en" codecs="mp4a.40.2">
    <Role schemeIdUri="urn:mpeg:dash:role:2011" value="alternate"/><Role schemeIdUri="urn:mpeg:dash:role:2011" value="dub"/>
    <Label>English (auto-dubbed)</Label>
    <Representation id="140-en.2" bandwidth="128000"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=140-en.2</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
  </AdaptationSet>
  <AdaptationSet id="video" mimeType="video/mp4" contentType="video" codecs="avc1.640028">
    <Representation id="137" bandwidth="1000000" width="1920" height="1080"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=137</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
  </AdaptationSet>
  <AdaptationSet id="original" mimeType="audio/mp4" contentType="audio" lang="ja" codecs="mp4a.40.2">
    <Role schemeIdUri="urn:mpeg:dash:role:2011" value="main"/><Label>日本語 (オリジナル)</Label>
    <Representation id="139-ja.4" bandwidth="48000"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=139-ja.4</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
    <Representation id="140-ja.4" bandwidth="128000"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=140-ja.4</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
  </AdaptationSet>
  <AdaptationSet id="ja-dub" mimeType="audio/mp4" contentType="audio" lang="ja" codecs="mp4a.40.2">
    <Role schemeIdUri="urn:mpeg:dash:role:2011" value="alternate"/><Role schemeIdUri="urn:mpeg:dash:role:2011" value="dub"/><Label>日本語 (吹き替え)</Label>
    <Representation id="140-ja.3" bandwidth="128000"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=140-ja.3</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
  </AdaptationSet>
  <AdaptationSet id="description" mimeType="audio/mp4" contentType="audio" lang="ja" codecs="mp4a.40.2">
    <Role schemeIdUri="urn:mpeg:dash:role:2011" value="alternate"/><Role schemeIdUri="urn:mpeg:dash:role:2011" value="description"/><Label>日本語 (音声解説)</Label>
    <Representation id="140-ja.5" bandwidth="128000"><BaseURL>/companion/videoplayback?expire=EXPIRY&amp;host=rr1---sn-test.googlevideo.com&amp;id=140-ja.5</BaseURL><SegmentBase indexRange="100-200"><Initialization range="0-99"/></SegmentBase></Representation>
  </AdaptationSet>
</Period></MPD>`
	r, err := s.parseManifest([]byte(strings.ReplaceAll(input, "EXPIRY", expiry)))
	if err != nil {
		t.Fatal(err)
	}
	const roleScheme = "urn:mpeg:dash:role:2011"
	wantSets := []adaptation{
		{ID: "en-dub", ContentType: "audio", Lang: "en", Label: "English (auto-dubbed)", Roles: []role{{roleScheme, "alternate"}, {roleScheme, "dub"}}, Representations: []representation{{ID: "140-en.2", Bandwidth: 128000}}},
		{ID: "video", ContentType: "video", Representations: []representation{{ID: "137", Bandwidth: 1000000}}},
		{ID: "original", ContentType: "audio", Lang: "ja", Label: "日本語 (オリジナル)", Roles: []role{{roleScheme, "main"}}, Representations: []representation{{ID: "139-ja.4", Bandwidth: 48000}, {ID: "140-ja.4", Bandwidth: 128000}}},
		{ID: "ja-dub", ContentType: "audio", Lang: "ja", Label: "日本語 (吹き替え)", Roles: []role{{roleScheme, "alternate"}, {roleScheme, "dub"}}, Representations: []representation{{ID: "140-ja.3", Bandwidth: 128000}}},
		{ID: "description", ContentType: "audio", Lang: "ja", Label: "日本語 (音声解説)", Roles: []role{{roleScheme, "alternate"}, {roleScheme, "description"}}, Representations: []representation{{ID: "140-ja.5", Bandwidth: 128000}}},
	}
	for _, tc := range []struct {
		name string
		r    *resolved
		sets []adaptation
	}{
		{"video", r, wantSets},
		{"audio only", r.audioOnly(), []adaptation{wantSets[0], wantSets[2], wantSets[3], wantSets[4]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := tc.r.manifest("audio-tracks")
			if err != nil {
				t.Fatal(err)
			}
			var got mpd
			if err := xml.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Periods) != 1 || len(got.Periods[0].Sets) != len(tc.sets) {
				t.Fatalf("lost or unexpected tracks: %s", data)
			}
			resourceIndex := 0
			for i, want := range tc.sets {
				set := got.Periods[0].Sets[i]
				if set.ID != want.ID || set.ContentType != want.ContentType || set.Lang != want.Lang || set.Label != want.Label || !slices.Equal(set.Roles, want.Roles) {
					t.Fatalf("track metadata changed: got %+v, want %+v", set, want)
				}
				if len(set.Representations) != len(want.Representations) {
					t.Fatalf("track %s lost bitrate choices", want.ID)
				}
				for j, rep := range set.Representations {
					wantRep := want.Representations[j]
					if rep.ID != wantRep.ID || rep.Bandwidth != wantRep.Bandwidth {
						t.Fatalf("track %s representation changed: %+v", want.ID, rep)
					}
					if wantURL := fmt.Sprintf("/api/playback/audio-tracks/media/%d", resourceIndex); rep.BaseURL != wantURL {
						t.Fatalf("track %s media URL = %s, want %s", want.ID, rep.BaseURL, wantURL)
					}
					if resourceIndex >= len(tc.r.Resources) {
						t.Fatalf("track %s has no media resource", want.ID)
					}
					wantResource := resource{
						URL:  f.server.URL + "/companion/videoplayback?expire=" + expiry + "&host=rr1---sn-test.googlevideo.com&id=" + wantRep.ID,
						Kind: want.ContentType,
					}
					if res := tc.r.Resources[resourceIndex]; res != wantResource {
						t.Fatalf("track %s resource = %+v, want %+v", want.ID, res, wantResource)
					}
					resourceIndex++
				}
			}
			if len(tc.r.Resources) != resourceIndex {
				t.Fatalf("exposed %d resources for %d representations", len(tc.r.Resources), resourceIndex)
			}
		})
	}
}
