package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/youtube"
	"github.com/cufee/feedlr-yt/internal/database"
	"github.com/cufee/feedlr-yt/internal/logic"
	"github.com/cufee/feedlr-yt/internal/sessions"
	"github.com/gofiber/fiber/v3"
	"github.com/microcosm-cc/bluemonday"
)

type frontendChunk struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
}

type frontendUI struct {
	assets        fs.FS
	db            database.Client
	sessions      *sessions.SessionClient
	entry         string
	styles        []string
	preloads      []string
	devURL        string
	metadata      func(context.Context, string, string) (frontendMetadata, error)
	metadataSlots chan struct{}
}

func newFrontendUI(assets fs.FS, db database.Client, sc *sessions.SessionClient) (*frontendUI, error) {
	u := &frontendUI{assets: assets, db: db, sessions: sc, metadataSlots: make(chan struct{}, 4)}
	if db != nil {
		u.metadata = u.loadProviderMetadata
	}
	if devURL := strings.TrimRight(os.Getenv("FEEDLR_VITE_URL"), "/"); devURL != "" {
		parsed, err := url.Parse(devURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("invalid FEEDLR_VITE_URL")
		}
		u.devURL = devURL
		return u, nil
	}
	data, err := fs.ReadFile(assets, "assets/ui/manifest.json")
	if err != nil {
		return nil, fmt.Errorf("load frontend manifest (run npm run build): %w", err)
	}
	var manifest map[string]frontendChunk
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode frontend manifest: %w", err)
	}
	entry, ok := manifest["index.html"]
	if !ok || entry.File == "" {
		return nil, fmt.Errorf("frontend manifest has no index.html entry")
	}
	seenFiles := map[string]bool{}
	seenCSS := map[string]bool{}
	var collect func(string) error
	collect = func(key string) error {
		if seenFiles[key] {
			return nil
		}
		seenFiles[key] = true
		chunk, ok := manifest[key]
		if !ok || !fs.ValidPath(chunk.File) {
			return fmt.Errorf("invalid frontend chunk %q", key)
		}
		if _, err := fs.Stat(assets, "assets/ui/"+chunk.File); err != nil {
			return fmt.Errorf("missing frontend chunk: %w", err)
		}
		for _, dependency := range chunk.Imports {
			if err := collect(dependency); err != nil {
				return err
			}
		}
		for _, css := range chunk.CSS {
			if !fs.ValidPath(css) {
				return fmt.Errorf("invalid frontend stylesheet")
			}
			if _, err := fs.Stat(assets, "assets/ui/"+css); err != nil {
				return fmt.Errorf("missing frontend stylesheet: %w", err)
			}
			if !seenCSS[css] {
				u.styles = append(u.styles, "/ui-assets/"+css)
				seenCSS[css] = true
			}
		}
		if key != "index.html" {
			u.preloads = append(u.preloads, "/ui-assets/"+chunk.File)
		}
		return nil
	}
	if err := collect("index.html"); err != nil {
		return nil, err
	}
	u.entry = "/ui-assets/" + entry.File
	return u, nil
}

// Vite supplies content-hashed paths. Do not apply the legacy per-file redirect:
// changing a chunk's directory breaks relative imports of its sibling chunks.
func (u *frontendUI) asset(c fiber.Ctx) error {
	// A request for a new build can reach a replica running the previous build.
	// Never let that temporary miss become a cached CDN error for the new hash.
	setFrontendAssetCacheControl(c, "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	if c.Method() != fiber.MethodGet && c.Method() != fiber.MethodHead {
		c.Set("Allow", "GET, HEAD")
		return c.SendStatus(fiber.StatusMethodNotAllowed)
	}
	name := c.Params("*")
	if !fs.ValidPath(name) || strings.HasPrefix(name, ".") {
		return c.SendStatus(fiber.StatusNotFound)
	}
	data, err := fs.ReadFile(u.assets, "assets/ui/"+name)
	if err != nil {
		return c.SendStatus(fiber.StatusNotFound)
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		c.Set("Content-Type", contentType)
	}
	if strings.HasPrefix(name, "assets/") {
		setFrontendAssetCacheControl(c, "public, max-age=31536000, immutable")
	} else {
		setFrontendAssetCacheControl(c, "no-cache")
	}
	return c.Send(data)
}

func setFrontendAssetCacheControl(c fiber.Ctx, value string) {
	c.Set("Cache-Control", value)
	c.Set("CDN-Cache-Control", value)
	c.Set("Cloudflare-CDN-Cache-Control", value)
}

type frontendDocument struct {
	Title, Description, Image, Kind, Author string
	Canonical, ISODuration                  string
	Duration                                int64
	Styles, Preloads                        []string
	Entry, DevURL                           string
	Maintenance                             bool
}

var frontendTemplate = template.Must(template.New("frontend").Parse(`<!doctype html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1, interactive-widget=resizes-content"><meta name="color-scheme" content="dark"><meta name="theme-color" content="#101014">
<title data-feedlr-head>{{.Title}}</title><meta data-feedlr-head name="description" content="{{.Description}}"><meta data-feedlr-head property="og:title" content="{{.Title}}"><meta data-feedlr-head property="og:type" content="{{.Kind}}"><meta data-feedlr-head property="og:description" content="{{.Description}}"><meta data-feedlr-head property="og:image" content="{{.Image}}"><meta data-feedlr-head name="twitter:card" content="summary_large_image"><meta data-feedlr-head name="twitter:title" content="{{.Title}}"><meta data-feedlr-head name="twitter:description" content="{{.Description}}"><meta data-feedlr-head name="twitter:image" content="{{.Image}}"><meta data-feedlr-head name="twitter:image:alt" content="{{.Title}}">{{if .Canonical}}<link data-feedlr-head rel="canonical" href="{{.Canonical}}"><meta data-feedlr-head property="og:url" content="{{.Canonical}}">{{end}}{{if .Author}}<meta data-feedlr-head name="author" content="{{.Author}}">{{end}}{{if .Duration}}<meta data-feedlr-head property="video:duration" content="{{.Duration}}"><meta data-feedlr-head property="og:video:duration" content="{{.Duration}}"><meta data-feedlr-head itemprop="duration" content="{{.ISODuration}}">{{end}}
<link rel="icon" href="/assets/favicon.png"><link rel="manifest" href="/assets/app.webmanifest">{{range .Styles}}<link rel="stylesheet" href="{{.}}">{{end}}{{range .Preloads}}<link rel="modulepreload" href="{{.}}">{{end}}
{{if .DevURL}}<script type="module">import RefreshRuntime from "{{.DevURL}}/@react-refresh"; RefreshRuntime.injectIntoGlobalHook(window); window.$RefreshReg$=()=>{}; window.$RefreshSig$=()=>type=>type; window.__vite_plugin_react_preamble_installed__=true;</script><script type="module" src="{{.DevURL}}/@vite/client"></script><script type="module" src="{{.DevURL}}/src/main.tsx"></script>{{else if not .Maintenance}}<script type="module" src="{{.Entry}}"></script>{{end}}
</head><body>{{if .Maintenance}}<main style="max-width:42rem;margin:15vh auto;padding:2rem;font:1rem/1.7 system-ui;color:#f4f3f7;background:#1c1c23;border-radius:1rem"><h1>We’ll be right back</h1><p>Feedlr is taking a short break for maintenance. Please try again soon.</p></main>{{else}}<div id="root"></div><noscript><main><h1>Feedlr</h1><p>Enable JavaScript to use your feed and media player.</p></main></noscript>{{end}}</body></html>`))

func (u *frontendUI) page(c fiber.Ctx) error {
	if (c.Path() == "/" || c.Path() == "/login") && u.signedIn(c) {
		return c.Redirect().Status(fiber.StatusFound).To("/app")
	}
	doc := frontendDocument{Title: "Feedlr · Your favorites, at your pace", Description: "A thoughtful home for the YouTube channels and podcasts you choose.", Image: c.BaseURL() + "/assets/og.jpg", Kind: "website", Entry: u.entry, Styles: u.styles, Preloads: u.preloads, DevURL: u.devURL}
	doc.Canonical = c.BaseURL() + c.Path()
	if id := c.Params("id"); id != "" && u.metadata != nil && (strings.HasPrefix(c.Path(), "/video/") || strings.HasPrefix(c.Path(), "/channel/")) {
		kind := "channel"
		if strings.HasPrefix(c.Path(), "/video/") {
			kind = "video"
		}
		if metadata, ok := u.publicMetadata(c.Context(), kind, strings.Clone(id)); ok {
			doc.Title = html.UnescapeString(metadata.title) + " · Feedlr"
			doc.Description = plainDescription(metadata.description)
			doc.Author = html.UnescapeString(metadata.author)
			doc.Image = c.BaseURL() + metadata.image
			if kind == "video" {
				doc.Kind, doc.Duration = "video.episode", metadata.duration
				doc.ISODuration = fmt.Sprintf("PT%dS", metadata.duration)
			}
		}
	}
	return renderFrontend(c, doc)
}

type frontendMetadata struct {
	title, description, image, author string
	duration                          int64
}

func plainDescription(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(bluemonday.StrictPolicy().Sanitize(value))), " ")
}

// Cached public metadata remains available even when upstream requests occupy
// every provider worker. The shared deadline also bounds context-aware DB reads.
func (u *frontendUI) publicMetadata(parent context.Context, kind, id string) (frontendMetadata, bool) {
	ctx, cancel := context.WithTimeout(parent, 1500*time.Millisecond)
	defer cancel()
	if u.db != nil {
		cached, err := u.loadCachedMetadata(ctx, kind, id)
		if err == nil {
			return cached, true
		}
		if !database.IsErrNotFound(err) {
			return frontendMetadata{}, false
		}
	}
	if u.metadata == nil || ctx.Err() != nil {
		return frontendMetadata{}, false
	}
	// Legacy video provider calls can outlive the document deadline. Bound only
	// those calls, so a slow upstream cannot starve local cache hits.
	select {
	case u.metadataSlots <- struct{}{}:
	default:
		return frontendMetadata{}, false
	}
	type result struct {
		value frontendMetadata
		err   error
	}
	ready := make(chan result, 1)
	go func() {
		defer func() { <-u.metadataSlots }()
		value, err := u.metadata(ctx, kind, id)
		ready <- result{value, err}
	}()
	select {
	case response := <-ready:
		return response.value, response.err == nil
	case <-ctx.Done():
		return frontendMetadata{}, false
	}
}

func (u *frontendUI) loadCachedMetadata(ctx context.Context, kind, id string) (frontendMetadata, error) {
	if kind == "video" {
		video, err := u.db.GetVideoByID(ctx, id)
		if err != nil {
			return frontendMetadata{}, err
		}
		metadata := frontendMetadata{title: video.Title, description: video.Description, duration: video.Duration, image: "/thumb/video/" + url.PathEscape(id) + "/maxresdefault"}
		if channel, err := u.db.GetChannel(ctx, video.ChannelID); err == nil {
			metadata.author = channel.Title
		}
		return metadata, nil
	}
	channel, err := u.db.GetChannel(ctx, id)
	if err != nil {
		return frontendMetadata{}, err
	}
	return frontendMetadata{title: channel.Title, description: channel.Description, image: "/thumb/channel/" + url.PathEscape(id)}, nil
}

func (u *frontendUI) loadProviderMetadata(ctx context.Context, kind, id string) (frontendMetadata, error) {
	if youtube.DefaultClient == nil || (kind == "video" && strings.HasPrefix(id, "pe_")) || (kind == "channel" && strings.HasPrefix(id, "ps_")) {
		return frontendMetadata{}, fmt.Errorf("public metadata provider unavailable")
	}
	if kind == "video" {
		video, err := logic.GetVideoByID(ctx, u.db, id)
		return frontendMetadata{title: video.Title, description: video.Description, author: video.Channel.Title, duration: int64(video.Duration), image: "/thumb/video/" + url.PathEscape(id) + "/maxresdefault"}, err
	}
	channel, _, err := logic.CacheChannel(ctx, u.db, id)
	if err != nil {
		return frontendMetadata{}, err
	}
	return frontendMetadata{title: channel.Title, description: channel.Description, image: "/thumb/channel/" + url.PathEscape(id)}, nil
}

func (u *frontendUI) signedIn(c fiber.Ctx) bool {
	s, ok := c.Locals("session").(sessions.Session)
	if !ok && u.sessions != nil && c.Cookies("session_id") != "" {
		s, _ = u.sessions.GetReadOnly(c.Context(), c.Cookies("session_id"))
	}
	id, valid := s.UserID()
	return valid && id != ""
}

func renderFrontend(c fiber.Ctx, doc frontendDocument) error {
	var body bytes.Buffer
	if err := frontendTemplate.Execute(&body, doc); err != nil {
		return err
	}
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(body.Bytes())
}

func (u *frontendUI) maintenance(c fiber.Ctx) error {
	if os.Getenv("MAINTENANCE_MODE") != "true" {
		return c.Next()
	}
	c.Set("Retry-After", "60")
	if strings.HasPrefix(c.Path(), "/api/") {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": fiber.Map{"code": "maintenance", "message": "Feedlr is taking a short break. Please try again soon."}})
	}
	c.Status(fiber.StatusServiceUnavailable)
	return renderFrontend(c, frontendDocument{Title: "Feedlr · Back soon", Description: "Feedlr is taking a short break for maintenance.", Styles: u.styles, Maintenance: true})
}
