# Database Guide

## Overview

The application uses **SQLite3** with **WAL mode** for better concurrent read performance.

**ORM:** [SQLBoiler](https://github.com/volatiletech/sqlboiler) - Generates type-safe Go code from the database schema.

## Configuration

Connection string options:
```go
"file://./data.db?_fk=1&_auto_vacuum=2&_synchronous=1&_journal_mode=WAL"
```

| Option | Value | Purpose |
|--------|-------|---------|
| `_fk` | 1 | Enable foreign key constraints |
| `_auto_vacuum` | 2 | Incremental vacuum |
| `_synchronous` | 1 | Normal sync (balanced) |
| `_journal_mode` | WAL | Write-Ahead Logging |

## Schema

### Users
```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    username TEXT NOT NULL UNIQUE,
    permissions TEXT NOT NULL DEFAULT ''
);
```

### Channels
```sql
CREATE TABLE channels (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    thumbnail TEXT NOT NULL,
    uploads_playlist_id TEXT,
    feed_updated_at DATE
);
```

### Videos
```sql
CREATE TABLE videos (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    duration INTEGER NOT NULL,
    published_at DATE NOT NULL,
    private BOOLEAN NOT NULL DEFAULT FALSE,
    type TEXT NOT NULL DEFAULT 'video',
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE
);

-- Indices
CREATE INDEX idx_videos_published_at ON videos(published_at);
CREATE INDEX idx_videos_channel_id ON videos(channel_id);
CREATE INDEX idx_videos_published_at_channel_id ON videos(published_at, channel_id);
```

### Views (Watch History)
```sql
CREATE TABLE views (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    video_id TEXT NOT NULL REFERENCES videos(id),
    progress INTEGER NOT NULL DEFAULT 0,
    hidden BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX idx_views_video_id_user_id ON views(video_id, user_id);
```

### Subscriptions
```sql
CREATE TABLE subscriptions (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    favorite BOOLEAN NOT NULL DEFAULT FALSE,
    channel_id TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_subscriptions_user_id_channel_id ON subscriptions(user_id, channel_id);
```

### Sessions
```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    connection_id TEXT,
    expires_at DATE NOT NULL,
    last_used DATE NOT NULL,
    deleted BOOLEAN NOT NULL DEFAULT FALSE,
    meta BLOB
);
```

### Settings
```sql
CREATE TABLE settings (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    data BLOB NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);
```

### Passkeys
```sql
CREATE TABLE passkeys (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    data BLOB NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);
```

### App Configuration
```sql
CREATE TABLE app_configuration (
    id TEXT PRIMARY KEY,
    created_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    version INTEGER NOT NULL DEFAULT 0,
    data BLOB
);
```

## Query Patterns

### Functional Options Pattern

Queries use functional options for flexible filtering:

```go
// Find videos with multiple options
videos, err := db.FindVideos(ctx,
    Video.WithChannel(),              // Join channel data
    Video.TypeNot("short", "private"),// Exclude types
    Video.Channel(channelIDs...),     // Filter by channels
    Video.Limit(50),                  // Limit results
)

// Get user subscriptions with eager loading
subs, err := db.UserSubscriptions(ctx, userID,
    Subscription.WithChannel(),
)

// Get channel with latest videos
channel, err := db.GetChannel(ctx, channelID,
    Channel.WithVideos(10),  // Load 10 latest videos
)
```

### Available Query Options

**Videos:**
```go
Video.WithChannel()         // Eager load channel
Video.Channel(ids...)       // Filter by channel IDs
Video.TypeEq(types...)      // Include only these types
Video.TypeNot(types...)     // Exclude these types
Video.ID(ids...)            // Filter by video IDs
Video.Limit(n)              // Limit results
Video.Select(cols...)       // Select specific columns
```

**Channels:**
```go
Channel.WithVideos(limit)   // Load latest N videos
Channel.WithSubscriptions() // Load subscription data
Channel.ID(ids...)          // Filter by IDs
```

**Subscriptions:**
```go
Subscription.WithChannel()  // Eager load channel
Subscription.WithUser()     // Eager load user
```

### Upsert Operations

```go
// Upsert video (insert or update on conflict)
video := &models.Video{
    ID:          videoID,
    Title:       title,
    Duration:    duration,
    PublishedAt: publishedAt,
    ChannelID:   channelID,
}

err := video.Upsert(ctx, db, true,
    []string{models.VideoColumns.ID},           // Conflict column
    boil.Blacklist(models.VideoColumns.CreatedAt), // Don't update
    boil.Infer(),                               // Update rest
)
```

### Common Operations

**Create user:**
```go
user, err := db.CreateUser(ctx, userID, username)
```

**Find user by username:**
```go
user, err := db.FindUser(ctx, username)
```

**Get user's watch history:**
```go
views, err := db.GetRecentUserViews(ctx, userID, 50)
```

**Update view progress:**
```go
view := &models.View{
    ID:       viewID,
    UserID:   userID,
    VideoID:  videoID,
    Progress: 120, // seconds
}
err := db.UpsertView(ctx, view)
```

**Create subscription:**
```go
sub, err := db.NewSubscription(ctx, userID, channelID)
```

**Delete subscription:**
```go
err := db.DeleteSubscription(ctx, userID, channelID)
```

## Error Handling

```go
import "github.com/user/youtube-app/internal/database"

// Check for not found
if database.IsErrNotFound(err) {
    // Handle missing record
}
```

## Migrations

**Tool:** [Atlas](https://atlasgo.io/)

**Location:** `internal/database/migrations/`

**Apply migrations:**
```bash
atlas migrate apply --url "sqlite://./data.db"
```

**Create new migration:**
```bash
atlas migrate diff migration_name --to "file://schema.sql"
```

## Generating Models

After schema changes, regenerate SQLBoiler models:

```bash
task generate
# or
sqlboiler sqlite3
```

Configuration in `sqlboiler.toml`:
```toml
output = "internal/database/models"
wipe = true
no-tests = false
add-enum-types = true
```

## File Reference

| Purpose | Path |
|---------|------|
| Client interface | `internal/database/client.go` |
| SQLite implementation | `internal/database/sqlite.go` |
| Query options | `internal/database/*.go` |
| Generated models | `internal/database/models/` |
| Migrations | `internal/database/migrations/` |
| Config | `sqlboiler.toml` |
