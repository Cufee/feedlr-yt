# [Feedlr](https://feedlr.app)
Feedlr is an alternative frontend for YouTube. The main goal of this project is to make following you favorite creators simpler and to reduce the amount of doom scrolling.  

Feedlr is using the following approach to achieve this:
- There is no channel discovery - you will need to know who you want to follow
- There are no recommendation, play next or related videos
- Only the latest 3 videos for each channel are visible in your feed, shorts are excluded from the feed
- Channels without any new videos are hidden from the main view
- Embedded ads can be skipped using an integration with SponsorBlock

## Current State
The core functionality is fully complete and is working reliably.

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