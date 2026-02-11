# Frontend Development Guide

For the Tailwind-first Liquid Glass redesign direction and full migration inventory/plan, see `docs/LIQUID-GLASS-REDESIGN.md`.

## Template System (Templ)

This project uses [Templ](https://templ.guide/) - a type-safe templating language for Go that compiles to Go code.

### Build Process

```bash
go generate ./...  # Compiles *.templ → *_templ.go
task generate      # Or use the task runner
```

### Directory Structure

```
internal/templates/
├── layouts/              # Page layouts
│   ├── main.templ        # Unauthenticated pages
│   ├── app.templ         # Authenticated app pages
│   ├── video.templ       # Video player fullscreen
│   ├── blank.templ       # Minimal layout
│   └── partials/         # Shared layout parts
│       ├── head.templ    # CSS/JS includes
│       ├── navbar.templ  # Navigation bar
│       └── progress.templ # HTMX progress bar
├── pages/                # Full pages
│   ├── landing.templ
│   ├── login.templ
│   ├── video.templ
│   └── app/              # Authenticated pages
│       ├── index.templ
│       ├── settings.templ
│       ├── subscriptions.templ
│       └── history.templ
└── components/           # Reusable components
    ├── feed/             # Video grids/carousels
    ├── icons/            # SVG icon components
    ├── shared/           # Logo, Search, etc.
    └── settings/         # Settings components
```

## UI Primitives (Phase 01-02)

Reusable primitives are now being introduced under `internal/templates/components/ui/`.

Current primitives:

- `button.templ` (`Button`, variants/sizes)
- `input.templ` (`Input`, `SearchInput`)
- `layout.templ` (`Card`, `Section`, `Badge`, `EmptyState`)
- `tabs.templ` (`Tabs`, `TabButton`)
- `toggle.templ` (`Toggle`, `ToggleWithLabel`)
- `dialog.templ` (`Dialog`, `DialogBackdropButton`)
- `toast.templ` (`Toast`)
- `pageshell.templ` (`PageShellMain`, `PageShellFooter`, `PageShellVideo`)
- `navbar.templ` (`Navbar`, guest/authed variants)
- `footer.templ` (`Footer`)
- `progress.templ` (`NavProgress`)

Supporting helpers:

- `internal/templates/components/ui/classes.go`

Usage example:

```templ
import "github.com/cufee/feedlr-yt/internal/templates/components/ui"

@ui.Section("Profile")
@ui.Card() {
  @ui.Input("display-name", "display_name", "", "Display name")
  @ui.Button("Save", ui.WithButtonVariant(ui.ButtonPrimary))
}
```

Guidance:

- Prefer Tailwind utility composition and tokenized classes over one-off CSS.
- Prefer `ui` primitives before creating new ad-hoc component styles.

## Adding a New Page

### 1. Create the Template

Create `internal/templates/pages/app/mypage.templ`:

```templ
package app

import "github.com/user/youtube-app/internal/types"

templ MyPage(props types.MyPageProps) {
    <div class="flex flex-col gap-4">
        <h1 class="text-2xl font-bold">{ props.Title }</h1>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            for _, item := range props.Items {
                @itemCard(item)
            }
        </div>
    </div>
}

templ itemCard(item types.Item) {
    <div class="p-4 rounded-lg bg-base-200">
        { item.Name }
    </div>
}
```

### 2. Define Props

Add to `internal/types/props.go`:

```go
type MyPageProps struct {
    Title string
    Items []Item
}

type Item struct {
    ID   string
    Name string
}
```

### 3. Create Route Handler

Create `internal/server/routes/app/mypage.go`:

```go
package app

import (
    "net/http"

    "github.com/user/youtube-app/internal/server/handler"
    "github.com/user/youtube-app/internal/templates/layouts"
    "github.com/user/youtube-app/internal/templates/pages/app"
    "github.com/a]templ"
    "github.com/vovkos/brewed"
)

var MyPage brewed.Page[*handler.Context] = func(ctx *handler.Context) (
    brewed.Layout[*handler.Context],
    templ.Component,
    error,
) {
    // Check authentication
    userID, ok := ctx.UserID()
    if !ok {
        return nil, nil, ctx.Redirect("/login", http.StatusTemporaryRedirect)
    }

    // Fetch data
    props, err := logic.GetMyPageProps(ctx.Context(), ctx.Database(), userID)
    if err != nil {
        return nil, nil, ctx.Err(err)
    }

    // Return layout + component
    return layouts.App, app.MyPage(*props), nil
}
```

### 4. Register Route

Add to `internal/server/server.go`:

```go
// In the /app route group
app := server.Group("/app").Use(limiterMiddleware).Use(authMw)
app.All("/mypage", toFiber(rapp.MyPage))
```

## Adding Components

### Simple Component

Create `internal/templates/components/shared/badge.templ`:

```templ
package shared

templ Badge(text string) {
    <span class="badge badge-primary">{ text }</span>
}
```

Usage:
```templ
@shared.Badge("New")
```

### Component with Options Pattern

For flexible, configurable components:

```templ
package shared

type cardOptions struct {
    showIcon    bool
    variant     string
    highlighted bool
}

type CardOption func(*cardOptions)

var WithIcon CardOption = func(o *cardOptions) {
    o.showIcon = true
}

func WithVariant(v string) CardOption {
    return func(o *cardOptions) { o.variant = v }
}

var Highlighted CardOption = func(o *cardOptions) {
    o.highlighted = true
}

func Card(title string, opts ...CardOption) templ.Component {
    var o cardOptions
    for _, apply := range opts {
        apply(&o)
    }
    return cardImpl(title, o)
}

templ cardImpl(title string, opts cardOptions) {
    <div class={ "card", templ.KV("card-bordered", opts.highlighted) }>
        if opts.showIcon {
            @icons.Star()
        }
        <div class="card-body">
            <h2 class="card-title">{ title }</h2>
        </div>
    </div>
}
```

Usage:
```templ
@shared.Card("My Card", shared.WithIcon, shared.WithVariant("primary"))
```

## JavaScript Integration

### Libraries

| Library | Purpose | Location |
|---------|---------|----------|
| HTMX | Server-driven UI updates | `/assets/js/htmx.min.js` |
| Hyperscript | Lightweight interactions | `/assets/js/hyperscript.min.js` |
| Fuse.js | Client-side search | CDN |

### Inline Scripts in Templ

```templ
script initPlayer(videoId string, startTime int) {
    const player = new YT.Player('player', {
        videoId: videoId,
        playerVars: { start: startTime }
    });
}

templ VideoPlayer(videoId string, startTime int) {
    <div id="player"></div>
    @shared.EmbedScript(initPlayer(videoId, startTime), videoId, startTime)
}
```

### HTMX Patterns

**Button with loading state:**
```html
<button
    hx-post="/api/action"
    hx-target="#result"
    hx-swap="innerHTML"
    hx-indicator="#spinner"
    class="btn btn-primary">
    Submit
</button>
<span id="spinner" class="loading loading-spinner htmx-indicator"></span>
```

**Form submission:**
```html
<form
    hx-post="/api/submit"
    hx-target="#form-container"
    hx-swap="outerHTML">
    <input name="query" class="input input-bordered" />
    <button class="btn btn-primary">Search</button>
</form>
```

**Infinite scroll:**
```html
<div
    hx-get="/api/more?page=2"
    hx-trigger="revealed"
    hx-swap="afterend">
    Loading more...
</div>
```

### Hyperscript Examples

**Toggle visibility:**
```html
<button _="on click toggle .hidden on #panel">Toggle</button>
```

**Conditional form submission:**
```html
<form _="on submit if #input.value == '' halt">
```

## Styling with Tailwind + DaisyUI

### Build CSS

```bash
npm run build  # One-time build
npm run dev    # Watch mode
```

### Configuration

`tailwind.css`:
```css
@import "tailwindcss";
@plugin "daisyui" {
    themes: dim --default;
}
```

### Theme

The app uses DaisyUI's **"dim"** theme (dark mode only).

Apply via: `<html data-theme="dim">`

### Common Components

**Buttons:**
```html
<button class="btn btn-primary">Primary</button>
<button class="btn btn-neutral">Neutral</button>
<button class="btn btn-ghost btn-sm">Small Ghost</button>
<button class="btn btn-square"><icon/></button>
```

**Form inputs:**
```html
<input class="input input-bordered" placeholder="Text" />
<input class="input input-bordered input-error" /> <!-- Error state -->
<textarea class="textarea textarea-bordered"></textarea>
<select class="select select-bordered">
    <option>Option 1</option>
</select>
```

**Input groups (join):**
```html
<div class="join">
    <input class="input input-bordered join-item" />
    <button class="btn btn-primary join-item">Search</button>
</div>
```

**Cards:**
```html
<div class="card bg-base-200">
    <div class="card-body">
        <h2 class="card-title">Title</h2>
        <p>Content</p>
    </div>
</div>
```

**Modal:**
```html
<dialog id="my_modal" class="modal modal-top lg:modal-middle">
    <div class="modal-box">
        <h3 class="font-bold text-lg">Title</h3>
        <p>Content</p>
    </div>
    <form method="dialog" class="modal-backdrop">
        <button>close</button>
    </form>
</dialog>

<!-- Open with: document.getElementById('my_modal').showModal() -->
```

### Responsive Patterns

```html
<!-- Grid: 1 col mobile, 2 cols tablet, 3 cols desktop -->
<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">

<!-- Hide on mobile, show on desktop -->
<div class="hidden md:block">

<!-- Full width mobile, max width desktop -->
<div class="w-full max-w-7xl mx-auto">
```

### Conditional Classes in Templ

```templ
// Using templ.KV
<div class={ "btn", templ.KV("btn-active", isActive) }>

// Using helper function
<input class={ "input input-bordered", shared.OptionalClass(!valid, "input-error") } />
```

## File Reference

| Purpose | Path |
|---------|------|
| Layouts | `internal/templates/layouts/*.templ` |
| Pages | `internal/templates/pages/**/*.templ` |
| Components | `internal/templates/components/**/*.templ` |
| Route handlers | `internal/server/routes/**/*.go` |
| Props types | `internal/types/props.go` |
| CSS source | `tailwind.css` |
| CSS output | `assets/css/style.css` |
| JS libraries | `assets/js/` |
