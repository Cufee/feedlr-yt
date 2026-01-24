# YouTube API Client Guide

## Overview

The YouTube client combines:
1. **YouTube Data API v3** - Channel search, metadata
2. **Desktop Player API** - Video details, playback info (undocumented)
3. **OAuth2 Device Flow** - User authentication for personalized features

## Location

```
internal/api/youtube/
├── client.go          # Main client wrapper
├── channel.go         # Channel operations
├── playlists.go       # Playlist video fetching
├── player.go          # Video details (abstraction)
├── player_desktop.go  # Desktop Player API implementation
├── setup.go           # Package initialization
└── auth/              # OAuth2 authentication
    ├── client.go      # Auth client
    ├── context.go     # Web player context
    └── ...
```

## Initialization

```go
import "github.com/user/youtube-app/internal/api/youtube"

// Create auth client (handles OAuth2)
authClient := youtube.NewAuthClient(dbConfigClient)

// Create YouTube client
ytClient, err := youtube.NewClient(apiKey, authClient)
```

## API Methods

### Search Channels

```go
channels, err := ytClient.SearchChannels(ctx, "channel name", 5)
// Returns: []youtube.Channel
```

**Channel struct:**
```go
type Channel struct {
    ID          string
    Title       string
    Description string
    Thumbnail   string
    URL         string
}
```

### Get Channel Details

```go
channel, err := ytClient.GetChannel(ctx, channelID)
```

### Get Channel Videos

```go
videos, err := ytClient.GetChannelVideos(
    ctx,
    channelID,
    uploadedAfter,  // time.Time - only videos after this date
    50,             // limit
    skipVideoIDs,   // []string - IDs to skip
)
```

**Flow:**
1. Gets channel's uploads playlist ID
2. Fetches playlist items
3. Gets video details for each item
4. Filters out shorts, private videos

### Get Video Details

```go
details, err := ytClient.GetVideoPlayerDetails(videoID)
```

**VideoDetails struct:**
```go
type VideoDetails struct {
    ID           string
    Title        string
    Description  string
    Duration     int        // seconds
    PublishedAt  time.Time
    Thumbnail    string
    Type         VideoType  // video, short, live, private, etc.
    IsLive       bool
    StreamingURL string     // For playback
}
```

**Video Types:**
```go
const (
    VideoTypeVideo          // Standard video
    VideoTypeShort          // Short (duration <= 60s or vertical)
    VideoTypeLiveStream     // Active live stream
    VideoTypeUpcomingStream // Scheduled stream
    VideoTypePrivate        // Private/unavailable
    VideoTypeFailed         // Login required
)
```

## Rate Limiting

The client enforces **5 requests/second** to the player API:

```go
var playerLimiter = time.NewTicker(time.Second / 5)

func (c *client) GetVideoPlayerDetails(videoId string) (*VideoDetails, error) {
    <-playerLimiter.C  // Wait for rate limiter
    return c.getDesktopPlayerDetails(videoId)
}
```

Playlist fetches use **3 concurrent goroutines** max for parallel video detail requests.

## Authentication

### OAuth2 Device Flow

The client uses Google's device flow for TV/limited-input devices:

1. **Extract credentials** - Fetches YouTube TV page, extracts client ID/secret
2. **Request device code** - Gets code for user to enter at google.com/device
3. **Poll for token** - Waits for user to authorize
4. **Auto-refresh** - Refreshes token before expiration

### Auth Client Usage

```go
authClient := youtube.NewAuthClient(dbConfigClient)

// Start device flow (if not authenticated)
deviceCode, err := authClient.StartDeviceFlow()
// Returns: code for user to enter, verification URL

// Check auth status
status := authClient.GetStatus()
// AuthStatusNotStarted, AuthStatusPendingApproval, AuthStatusAuthenticated, etc.

// Get authenticated HTTP client (for API requests)
httpClient := authClient.GetHTTPClient()
```

### Token Storage

Tokens are cached in the database:
- **Table:** `app_configuration`
- **Key:** `"youtube-oauth-store"`
- **Data:** JSON with token and client info

On startup, the client:
1. Loads cached tokens
2. Checks expiration
3. Refreshes if needed
4. Falls back to device flow if refresh fails

## Web Player Context

For the Desktop Player API, requests need context data:

```go
type WebPlayerRequestContext struct {
    ApiKey      string
    VisitorID   string  // Random 11-char ID
    ClientInfo  ClientContext
}
```

**Extracted from:** `https://www.youtube.com/sw.js_data`

Includes:
- API key
- Client version
- Visitor data
- Device info

## Error Handling

```go
details, err := ytClient.GetVideoPlayerDetails(videoID)
if err != nil {
    // Network error, rate limited, etc.
}

if details.Type == youtube.VideoTypePrivate {
    // Video is private/unavailable
}

if details.Type == youtube.VideoTypeFailed {
    // Requires login (age-restricted, etc.)
}
```

## Shorts Detection

Videos are classified as shorts if:
- Duration <= 60 seconds
- OR aspect ratio is vertical (width < height)

```go
func isShort(duration int, width, height int) bool {
    return duration <= 60 || width < height
}
```

## Example: Fetch New Videos

```go
func fetchNewVideos(ctx context.Context, yt *youtube.Client, channelID string) error {
    // Get channel's latest videos (last 7 days)
    since := time.Now().AddDate(0, 0, -7)

    videos, err := yt.GetChannelVideos(ctx, channelID, since, 50, nil)
    if err != nil {
        return err
    }

    for _, video := range videos {
        if video.Type == youtube.VideoTypeVideo {
            // Process regular videos only
            fmt.Printf("%s: %s (%d sec)\n",
                video.ID, video.Title, video.Duration)
        }
    }

    return nil
}
```

## File Reference

| Purpose | Path |
|---------|------|
| Client interface | `internal/api/youtube/client.go` |
| Channel operations | `internal/api/youtube/channel.go` |
| Playlist fetching | `internal/api/youtube/playlists.go` |
| Player API | `internal/api/youtube/player_desktop.go` |
| Auth client | `internal/api/youtube/auth/client.go` |
| Player context | `internal/api/youtube/auth/context.go` |
| Types | `internal/api/youtube/types.go` |
