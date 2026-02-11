# Architecture Overview

feedlr-yt is a self-hosted YouTube feed reader built with Go.

## Tech Stack

| Layer | Technology |
|-------|------------|
| **Backend** | Go 1.22+ with Fiber web framework |
| **Templating** | Templ (type-safe, compiles to Go) |
| **Frontend** | HTMX + Hyperscript (minimal JS) |
| **Styling** | Tailwind CSS v4 + shared `ui-*` primitives |
| **Database** | SQLite3 with SQLBoiler ORM |
| **Auth** | WebAuthn (passkeys) + Sessions |
| **YouTube** | YouTube Data API v3 + Desktop Player API |

## Project Structure

```
youtube-app/
├── main.go                 # Entry point, embeds assets
├── Taskfile.yaml           # Build tasks
├── tailwind.css            # Tailwind config
├── package.json            # CSS build scripts
├── assets/                 # Static files (embedded in binary)
│   ├── css/
│   └── js/
├── internal/
│   ├── api/
│   │   └── youtube/        # YouTube API client
│   │       └── auth/       # OAuth2 device flow
│   ├── database/           # Database layer
│   │   ├── models/         # SQLBoiler generated
│   │   └── migrations/     # Atlas migrations
│   ├── logic/              # Business logic
│   ├── server/             # HTTP server
│   │   ├── handler/        # Request context
│   │   └── routes/         # Route handlers
│   │       ├── app/        # Authenticated pages
│   │       └── api/        # API endpoints
│   ├── templates/          # Templ templates
│   │   ├── layouts/        # Page layouts
│   │   ├── pages/          # Full pages
│   │   └── components/     # Reusable components
│   └── types/              # Shared types/props
└── docs/                   # Documentation
```

## Request Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         HTTP Request                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Middleware Chain                            │
│  Logger → Favicon → Static → Outage → Cache → RateLimiter → Auth│
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Route Handler                              │
│         func(ctx) → (Layout, Component, error)                   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Templ Rendering                             │
│              Layout wraps Component → HTML                       │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       HTTP Response                              │
│                  Full page or HTMX fragment                      │
└─────────────────────────────────────────────────────────────────┘
```

## Key Abstractions

### Route Handlers

**Page routes** return a layout, component, and error:
```go
var MyPage brewed.Page[*handler.Context] = func(ctx *handler.Context) (
    brewed.Layout[*handler.Context],
    templ.Component,
    error,
) {
    // Auth check, data fetch, return (layout, component, nil)
}
```

**Partial routes** return just a component (for HTMX):
```go
var MyPartial brewed.Partial[*handler.Context] = func(ctx *handler.Context) (
    templ.Component,
    error,
) {
    // Return component fragment for HTMX swap
}
```

### Context

Custom context wraps Fiber with helpers:
```go
ctx.UserID()        // Get authenticated user ID
ctx.Database()      // Get database client
ctx.Session()       // Get session data
ctx.Query(key)      // Sanitized query param
ctx.Redirect(path)  // HTMX-aware redirect
```

### Props Pattern

Templates receive typed props structs:
```go
type VideoPlayerProps struct {
    Video             VideoProps
    ReportProgress    bool
    PlayerVolumeLevel int
    SkipSegments      []SegmentProps
    ReturnURL         string
}
```

## Build Commands

```bash
# Generate templates and models
task generate

# Build CSS
npm run build

# Run dev server
task dev

# Run tests
task test
```

## Related Documentation

- [Frontend Guide](./FRONTEND.md) - Templates, components, styling
- [Database Guide](./DATABASE.md) - Schema, queries, migrations
- [YouTube API Guide](./YOUTUBE-API.md) - Client, auth, rate limiting
- [Playlist Sync Design](./PLAYLIST-SYNC.md) - OAuth-based diff sync to a YouTube playlist
