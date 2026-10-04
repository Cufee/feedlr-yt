package playback

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestManifestStableVolumeChoices(t *testing.T) {
	type audio struct {
		label, language, mime, codec string
		roles, ids                   []string
		representationMetadata       bool
		foreignRoles                 bool
	}
	regular := audio{label: "English (Original)", language: "en", roles: []string{"main"}, ids: []string{"139-en.4", "140-en.4"}}
	stable := audio{label: "English (Original) (Stable Volume)", language: "en", roles: []string{"main", "enhanced-audio-intelligibility"}, ids: []string{"140-en.4-drc"}}
	withIDs := func(a audio, ids ...string) audio { a.ids = ids; return a }
	withRoles := func(a audio, roles ...string) audio { a.roles = roles; return a }
	withForeignRoles := func(a audio) audio { a.foreignRoles = true; return a }
	withLanguage := func(a audio, language string) audio { a.language = language; return a }
	withCodec := func(a audio, mime, codec string, representationMetadata bool) audio {
		a.mime, a.codec, a.representationMetadata = mime, codec, representationMetadata
		return a
	}
	for _, tc := range []struct {
		name       string
		tracks     []audio
		wantIDs    []string
		wantLabels []string
	}{
		{
			name:    "original stable volume precedes regular bitrates",
			tracks:  []audio{stable, regular},
			wantIDs: []string{"139-en.4", "140-en.4"}, wantLabels: []string{"English (Original)"},
		},
		{
			name:    "alternate stable role and different itag",
			tracks:  []audio{withIDs(regular, "139-en.4"), withRoles(stable, "alternate", "enhanced-audio-intelligibility")},
			wantIDs: []string{"139-en.4"}, wantLabels: []string{"English (Original)"},
		},
		{
			name:    "full regional track ID",
			tracks:  []audio{withIDs(stable, "140-en-US.4-drc"), withIDs(regular, "139-en-US.4")},
			wantIDs: []string{"139-en-US.4"}, wantLabels: []string{"English (Original)"},
		},
		{
			name: "human and AI recordings remain separate",
			tracks: []audio{
				{label: "Spanish (auto-dubbed) (Stable Volume)", language: "es", roles: []string{"alternate", "dub", "enhanced-audio-intelligibility"}, ids: []string{"140-es.3-drc"}},
				{label: "Spanish", language: "es", roles: []string{"alternate", "dub"}, ids: []string{"140-es.2"}},
			},
			wantIDs: []string{"140-es.3-drc", "140-es.2"}, wantLabels: []string{"Spanish (auto-dubbed)", "Spanish"},
		},
		{
			name:    "stable-only language remains available",
			tracks:  []audio{regular, {label: "日本語 (オリジナル) (Stable Volume)", language: "ja", roles: []string{"main", "enhanced-audio-intelligibility"}, ids: []string{"140-ja.4-drc"}}},
			wantIDs: []string{"139-en.4", "140-en.4", "140-ja.4-drc"}, wantLabels: []string{"English (Original)", "日本語 (オリジナル)"},
		},
		{
			name:    "different codec remains available",
			tracks:  []audio{regular, withCodec(withIDs(stable, "141-en.4-drc"), "audio/mp4", "mp4a.40.5", true)},
			wantIDs: []string{"139-en.4", "140-en.4", "141-en.4-drc"}, wantLabels: []string{"English (Original)", "English (Original)"},
		},
		{
			name:    "same codec in different containers remains available",
			tracks:  []audio{withCodec(withIDs(regular, "251-en.4"), "audio/mp4", "opus", false), withCodec(withIDs(stable, "251-en.4-drc"), "audio/webm", "opus", false)},
			wantIDs: []string{"251-en.4", "251-en.4-drc"}, wantLabels: []string{"English (Original)", "English (Original)"},
		},
		{
			name:    "representation metadata matches inherited metadata",
			tracks:  []audio{withCodec(stable, "audio/mp4", "mp4a.40.2", true), regular},
			wantIDs: []string{"139-en.4", "140-en.4"}, wantLabels: []string{"English (Original)"},
		},
		{
			name:    "voice boost remains distinct",
			tracks:  []audio{regular, {label: "English (Original) (Voice Boost)", language: "en", roles: []string{"main", "enhanced-audio-intelligibility"}, ids: []string{"140-en.4-vb"}}, withIDs(stable, "140-en.4-drc-vb")},
			wantIDs: []string{"139-en.4", "140-en.4", "140-en.4-vb"}, wantLabels: []string{"English (Original)", "English (Original) (Voice Boost)"},
		},
		{
			name:    "regular without voice boost cannot replace boosted DRC",
			tracks:  []audio{regular, withIDs(stable, "140-en.4-drc-vb")},
			wantIDs: []string{"139-en.4", "140-en.4", "140-en.4-drc-vb"}, wantLabels: []string{"English (Original)", "English (Original)"},
		},
		{
			name:    "unknown IDs retain their metadata",
			tracks:  []audio{withIDs(regular, "unrecognized"), withIDs(stable, "unrecognized-drc")},
			wantIDs: []string{"unrecognized", "unrecognized-drc"}, wantLabels: []string{"English (Original)", "English (Original) (Stable Volume)"},
		},
		{
			name:    "numeric DRC suffix without enhancement metadata is preserved",
			tracks:  []audio{regular, withRoles(stable, "main")},
			wantIDs: []string{"139-en.4", "140-en.4", "140-en.4-drc"}, wantLabels: []string{"English (Original)", "English (Original) (Stable Volume)"},
		},
		{
			name:    "foreign enhancement role is preserved",
			tracks:  []audio{regular, withForeignRoles(stable)},
			wantIDs: []string{"139-en.4", "140-en.4", "140-en.4-drc"}, wantLabels: []string{"English (Original)", "English (Original) (Stable Volume)"},
		},
		{
			name:    "unnamed recording pairs",
			tracks:  []audio{withIDs(stable, "140-drc"), withIDs(regular, "139")},
			wantIDs: []string{"139"}, wantLabels: []string{"English (Original)"},
		},
		{
			name:    "unnamed recordings in different languages stay separate",
			tracks:  []audio{withIDs(regular, "139"), withLanguage(withIDs(stable, "140-drc"), "ja")},
			wantIDs: []string{"139", "140-drc"}, wantLabels: []string{"English (Original)", "English (Original)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			s := newService(t, f)
			var input mpd
			if err := xml.Unmarshal([]byte(fixtureManifest(time.Now().Add(time.Hour).Unix())), &input); err != nil {
				t.Fatal(err)
			}
			audioTemplate, video := input.Periods[0].Sets[0], input.Periods[0].Sets[1]
			input.Periods[0].Sets = nil
			wantResources := make(map[string]resource)
			wantSets := make(map[string]adaptation)
			for i, track := range tc.tracks {
				a := adaptation{ID: fmt.Sprintf("audio-%d", i), ContentType: "audio", MIME: "audio/mp4", Codecs: "mp4a.40.2", Lang: track.language, Label: track.label}
				if track.mime != "" {
					a.MIME, a.Codecs = track.mime, track.codec
				}
				for _, value := range track.roles {
					scheme := "urn:mpeg:dash:role:2011"
					if track.foreignRoles {
						scheme = "urn:example:audio-role"
					}
					a.Roles = append(a.Roles, role{Scheme: scheme, Value: value})
				}
				for j, id := range track.ids {
					rep := audioTemplate.Representations[0]
					rep.ID, rep.Bandwidth, rep.Codecs = id, int64(48000+j*80000), ""
					if track.representationMetadata {
						rep.MIME, rep.Codecs = a.MIME, a.Codecs
					}
					rep.BaseURL += "&id=" + url.QueryEscape(id)
					a.Representations = append(a.Representations, rep)
					wantResources[id] = resource{URL: f.server.URL + rep.BaseURL, Kind: "audio"}
				}
				if track.representationMetadata {
					a.MIME, a.Codecs = "", ""
				}
				input.Periods[0].Sets = append(input.Periods[0].Sets, a)
				wantSets[a.ID] = a
				if i == 0 {
					// Keep video between audio sets so removing an earlier DRC set
					// exercises media indices for both playback modes.
					input.Periods[0].Sets = append(input.Periods[0].Sets, video)
					wantResources[video.Representations[0].ID] = resource{URL: f.server.URL + video.Representations[0].BaseURL, Kind: "video"}
				}
			}
			data, err := xml.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.parseManifest(data)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := xml.Marshal(r.MPD)
			for _, mode := range []struct {
				name string
				r    *resolved
			}{{"video", r}, {"audio only", r.audioOnly()}} {
				t.Run(mode.name, func(t *testing.T) {
					output, err := mode.r.manifest("stable-volume")
					if err != nil {
						t.Fatal(err)
					}
					var got mpd
					if err := xml.Unmarshal(output, &got); err != nil {
						t.Fatal(err)
					}
					var ids, labels []string
					resourceIndex, videoCount := 0, 0
					for _, a := range got.Periods[0].Sets {
						if want, isAudio := wantSets[a.ID]; isAudio {
							labels = append(labels, a.Label)
							if a.Lang != want.Lang || !reflect.DeepEqual(a.Roles, want.Roles) {
								t.Fatalf("recording metadata changed: got %+v, want %+v", a, want)
							}
						}
						for _, rep := range a.Representations {
							wantResource, ok := wantResources[rep.ID]
							if !ok || resourceIndex >= len(mode.r.Resources) || mode.r.Resources[resourceIndex] != wantResource {
								t.Fatalf("resource %d no longer maps to representation %s", resourceIndex, rep.ID)
							}
							if wantURL := fmt.Sprintf("/api/playback/stable-volume/media/%d", resourceIndex); rep.BaseURL != wantURL {
								t.Fatalf("representation %s URL = %q, want %q", rep.ID, rep.BaseURL, wantURL)
							}
							if wantResource.Kind == "audio" {
								ids = append(ids, rep.ID)
							} else {
								videoCount++
							}
							resourceIndex++
						}
					}
					if !slices.Equal(ids, tc.wantIDs) || !slices.Equal(labels, tc.wantLabels) {
						t.Fatalf("audio choices = %v / %v, want %v / %v", ids, labels, tc.wantIDs, tc.wantLabels)
					}
					if resourceIndex != len(mode.r.Resources) || (mode.name == "video" && videoCount != 1) || (mode.name == "audio only" && videoCount != 0) {
						t.Fatalf("incorrect resource count: representations=%d resources=%d video=%d", resourceIndex, len(mode.r.Resources), videoCount)
					}
				})
			}
			after, _ := xml.Marshal(r.MPD)
			if string(before) != string(after) {
				t.Fatal("audio-only conversion or session rewrite mutated the shared manifest")
			}
		})
	}
}
