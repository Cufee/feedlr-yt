# [Feedlr](https://feedlr.app)
Feedlr is an alternative frontend for YouTube. The main goal of this project is to make following you favorite creators simpler and to reduce the amount of doom scrolling.  

Feedlr is using the following approach to achieve this:
- There is no channel discovery - you will need to know who you want to follow
- There are no recommendation, play next or related videos
- Only the latest 3 videos for each channel are visible in your feed, shorts are excluded from the feed
- Channels without any new videos are hidden from the main view
- Embedded ads can be skipped using an integration with SponsorBlock

## Current State
I have finished the basic MVP and 90% of the functionality. There are a lot of UI elements missing and lorem needs to be replaced on the landing page.  
The next step from here would be a complete rewrite of all pages with [templ](https://github.com/a-h/templ) and organizing the backend logic to de-spaghettify it.

### Stack
- Go templates with [HTMX](htmx.org/), [Tailwind](https://tailwindcss.com/), [DaisyUI](https://daisyui.com/) and [Hyperscript](https://hyperscript.org/)
- Go Fiber
- Prisma
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