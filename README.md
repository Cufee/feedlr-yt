# [Feedlr](https://feedlr.app)
Feedlr is an alternative frontend for YouTube. The main goal of this project is to make following you favorite creators simpler and to reduce the amount of doom scrolling.  

Feedlr is using the following approach to achieve this:
- There is no channel discovery - you will need to know who you want to follow
- There are no recommendation, play next or related videos
- Your feed is limited, if you miss a video - it was probably not that important
- Embedded ads can be skipped using an integration with SponsorBlock

## Current State
The core functionality is fully complete and is working reliably.

#### Nice To Have
- A more descriptive landing page with examples
- Demo page that does not require login
- An onboarding flow with a tutorial on functionality and available settings
- Database cleanup / ttl indexes on Videos
- Native HTML video player
  - Video/Audio needs to be synced for streams better than 720p
- Channel View
  - Tiles for Subscriptions should open Channel View

### Stack
- Go [templ](https://templ.guide/) with [HTMX](https://htmx.org/), [Tailwind](https://tailwindcss.com/), [DaisyUI](https://daisyui.com/) and [Hyperscript](https://hyperscript.org/)
- Go Fiber
- MongoDB

### Developing
Start a local dev server:
```
task dev
```

Run all tests:
```
task test
```

Upgrade all packages
```
task upgrade
```