package playback

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Only the static, single-file SegmentBase profile emitted by Companion is
// accepted. Rebuilding this small schema prevents hidden external fetches via
// XLink, Location, SegmentTemplate, Initialization.sourceURL, or XML entities.
type mpd struct {
	XMLName   xml.Name `xml:"MPD"`
	XMLNS     string   `xml:"xmlns,attr"`
	Type      string   `xml:"type,attr"`
	Duration  string   `xml:"mediaPresentationDuration,attr"`
	MinBuffer string   `xml:"minBufferTime,attr,omitempty"`
	Profiles  string   `xml:"profiles,attr,omitempty"`
	Periods   []period `xml:"Period"`
}
type period struct {
	Sets []adaptation `xml:"AdaptationSet"`
}
type adaptation struct {
	ID              string           `xml:"id,attr,omitempty"`
	MIME            string           `xml:"mimeType,attr"`
	ContentType     string           `xml:"contentType,attr,omitempty"`
	Lang            string           `xml:"lang,attr,omitempty"`
	Codecs          string           `xml:"codecs,attr,omitempty"`
	Roles           []role           `xml:"Role"`
	Label           string           `xml:"Label,omitempty"`
	Representations []representation `xml:"Representation"`
}
type role struct {
	Scheme string `xml:"schemeIdUri,attr"`
	Value  string `xml:"value,attr"`
}
type representation struct {
	ID        string       `xml:"id,attr"`
	Bandwidth int64        `xml:"bandwidth,attr"`
	Codecs    string       `xml:"codecs,attr,omitempty"`
	MIME      string       `xml:"mimeType,attr,omitempty"`
	Width     int          `xml:"width,attr,omitempty"`
	Height    int          `xml:"height,attr,omitempty"`
	FrameRate string       `xml:"frameRate,attr,omitempty"`
	Sampling  string       `xml:"audioSamplingRate,attr,omitempty"`
	Channels  *role        `xml:"AudioChannelConfiguration,omitempty"`
	BaseURL   string       `xml:"BaseURL"`
	Segment   *segmentBase `xml:"SegmentBase"`
}
type segmentBase struct {
	IndexRange      string         `xml:"indexRange,attr"`
	IndexRangeExact string         `xml:"indexRangeExact,attr,omitempty"`
	Initialization  initialization `xml:"Initialization"`
}
type initialization struct {
	Range string `xml:"range,attr"`
}
type resource struct {
	URL  string
	Kind string
}
type resolved struct {
	MPD       mpd
	Resources []resource
	ExpiresAt time.Time
}

type audioTrackKey struct {
	ID, Language, MIME, Codec string
	VoiceBoost                bool
}

// YouTube.js names audio representations itag[-trackID][-drc][-vb]. Track IDs
// distinguish recordings in the same language; itags distinguish encodings.
// DRC and Voice Boost share a DASH role, so that role cannot identify DRC.
func (a adaptation) audioTrack(rep representation) (key audioTrackKey, drc, ok bool) {
	key.MIME = rep.MIME
	if key.MIME == "" {
		key.MIME = a.MIME
	}
	if !strings.HasPrefix(key.MIME, "audio/") {
		return key, false, false
	}
	key.Codec = rep.Codecs
	if key.Codec == "" {
		key.Codec = a.Codecs
	}
	// A regular track in another codec must not displace a stable-volume
	// encoding that may be the only one the browser can decode.
	id, vb := strings.CutSuffix(rep.ID, "-vb")
	id, drc = strings.CutSuffix(id, "-drc")
	if drc && !slices.Contains(a.Roles, role{Scheme: "urn:mpeg:dash:role:2011", Value: "enhanced-audio-intelligibility"}) {
		// An unfamiliar ID ending in -drc is not sufficient evidence to drop it.
		return key, false, false
	}
	itag, trackID, named := strings.Cut(id, "-")
	n, err := strconv.Atoi(itag)
	if err != nil || n <= 0 || (named && trackID == "") {
		return key, false, false
	}
	key.ID, key.Language, key.VoiceBoost = trackID, a.Lang, vb
	return key, drc, true
}

func (p period) withoutStableVolumeDuplicates() period {
	regular := make(map[audioTrackKey]bool)
	for _, a := range p.Sets {
		for _, rep := range a.Representations {
			if key, drc, ok := a.audioTrack(rep); ok && !drc {
				regular[key] = true
			}
		}
	}
	var filtered period
	for _, a := range p.Sets {
		kept := a
		kept.Representations = nil
		stableOnly := true
		for _, rep := range a.Representations {
			key, drc, ok := a.audioTrack(rep)
			if ok && drc && regular[key] {
				continue
			}
			stableOnly = stableOnly && ok && drc
			kept.Representations = append(kept.Representations, rep)
		}
		if len(kept.Representations) == 0 {
			continue
		}
		if stableOnly {
			// Preserve the recording's label (including original/dub/description)
			// while removing Companion's volume-processing suffix.
			kept.Label = strings.TrimSuffix(kept.Label, " (Stable Volume)")
		}
		filtered.Sets = append(filtered.Sets, kept)
	}
	return filtered
}

// qualities reports the complete video's dimensions even for audio-only
// sessions, so the browser can offer switching back to a video resolution.
func (r *resolved) qualities() []Quality {
	var qualities []Quality
	seen := make(map[Quality]bool)
	i := 0
	for _, p := range r.MPD.Periods {
		for _, a := range p.Sets {
			for _, rep := range a.Representations {
				quality := Quality{Width: rep.Width, Height: rep.Height}
				if r.Resources[i].Kind == "video" && quality.Width > 0 && quality.Height > 0 && !seen[quality] {
					qualities = append(qualities, quality)
					seen[quality] = true
				}
				i++
			}
		}
	}
	slices.SortFunc(qualities, func(a, b Quality) int {
		if a.Height != b.Height {
			if a.Height > b.Height {
				return -1
			}
			return 1
		}
		if a.Width > b.Width {
			return -1
		}
		if a.Width < b.Width {
			return 1
		}
		return 0
	})
	return qualities
}

func (r *resolved) audioOnly() *resolved {
	// Extraction results are shared by concurrent video/audio resolutions.
	// Allocate each changed slice rather than modifying that shared result.
	filtered := &resolved{MPD: r.MPD, ExpiresAt: r.ExpiresAt}
	filtered.MPD.Periods = make([]period, 0, len(r.MPD.Periods))
	i := 0
	for _, p := range r.MPD.Periods {
		var audioPeriod period
		for _, a := range p.Sets {
			audioSet := a
			audioSet.Representations = nil
			for _, rep := range a.Representations {
				res := r.Resources[i]
				i++
				if res.Kind != "audio" {
					continue
				}
				audioSet.Representations = append(audioSet.Representations, rep)
				filtered.Resources = append(filtered.Resources, res)
			}
			if len(audioSet.Representations) > 0 {
				audioSet.ContentType = "audio"
				if !strings.HasPrefix(audioSet.MIME, "audio/") {
					audioSet.MIME = audioSet.Representations[0].MIME
				}
				audioPeriod.Sets = append(audioPeriod.Sets, audioSet)
			}
		}
		filtered.MPD.Periods = append(filtered.MPD.Periods, audioPeriod)
	}
	return filtered
}

func (s *Service) parseManifest(data []byte) (*resolved, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if _, ok := t.(xml.Directive); ok {
			return nil, errors.New("XML directives unsupported")
		}
		if e, ok := t.(xml.StartElement); ok {
			switch e.Name.Local {
			case "SegmentTemplate", "SegmentList", "Location", "UTCTiming", "ContentProtection":
				return nil, errors.New("unsupported DASH profile")
			}
			for _, a := range e.Attr {
				if a.Name.Local == "href" || a.Name.Local == "sourceURL" {
					return nil, errors.New("external DASH resource")
				}
			}
		}
	}
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Type != "static" || len(m.Periods) != 1 || m.Duration == "" {
		return nil, errors.New("nonstatic manifest")
	}
	// Filter before constructing the media allowlist so manifest indices and
	// resources stay aligned in both video and audio-only sessions.
	m.Periods[0] = m.Periods[0].withoutStableVolumeDuplicates()
	m.XMLNS = "urn:mpeg:dash:schema:mpd:2011"
	r := &resolved{MPD: m}
	kinds := map[string]bool{}
	for _, p := range m.Periods {
		for _, a := range p.Sets {
			for _, rep := range a.Representations {
				mime := rep.MIME
				if mime == "" {
					mime = a.MIME
				}
				kind := strings.SplitN(mime, "/", 2)[0]
				if kind != "audio" && kind != "video" {
					return nil, errors.New("unsupported media type")
				}
				if rep.Segment == nil || !byteSpan.MatchString(rep.Segment.IndexRange) || !byteSpan.MatchString(rep.Segment.Initialization.Range) || rep.Bandwidth <= 0 || (rep.Codecs == "" && a.Codecs == "") {
					return nil, errors.New("invalid representation")
				}
				u, err := s.mediaURL(strings.TrimSpace(rep.BaseURL))
				if err != nil {
					return nil, err
				}
				exp, err := strconv.ParseInt(u.Query().Get("expire"), 10, 64)
				if err != nil {
					return nil, errors.New("missing URL expiry")
				}
				expires := time.Unix(exp, 0).Add(-5 * time.Second)
				if !expires.After(s.now().Add(2 * time.Minute)) {
					return nil, ErrExpired
				}
				if r.ExpiresAt.IsZero() || expires.Before(r.ExpiresAt) {
					r.ExpiresAt = expires
				}
				r.Resources = append(r.Resources, resource{u.String(), kind})
				kinds[kind] = true
				if len(r.Resources) > 200 {
					return nil, errors.New("too many representations")
				}
			}
		}
	}
	if !kinds["audio"] || !kinds["video"] {
		return nil, errors.New("audio or video missing")
	}
	return r, nil
}

func (s *Service) mediaURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("invalid media URL")
	}
	u = s.base.ResolveReference(u)
	if u.Scheme != s.base.Scheme || u.Host != s.base.Host || u.Path != s.base.Path+"/videoplayback" {
		return nil, errors.New("media origin not allowed")
	}
	// Companion's proxy chooses its destination from host; validate it too.
	hosts := u.Query()["host"]
	if len(hosts) != 1 || !googleVideoHost.MatchString(hosts[0]) {
		return nil, errors.New("media host not allowed")
	}
	return u, nil
}

func (r *resolved) manifest(sessionID string) ([]byte, error) {
	// A fresh decode keeps concurrent session rewrites isolated.
	data, err := xml.Marshal(r.MPD)
	if err != nil {
		return nil, err
	}
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	i := 0
	for p := range m.Periods {
		for a := range m.Periods[p].Sets {
			for v := range m.Periods[p].Sets[a].Representations {
				m.Periods[p].Sets[a].Representations[v].BaseURL = fmt.Sprintf("/api/playback/%s/media/%d", sessionID, i)
				i++
			}
		}
	}
	return xml.Marshal(m)
}
