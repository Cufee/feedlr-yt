package podcastindex

// DefaultClient is the process-wide PodcastIndex client, initialized at boot.
// It is nil when no API credentials are configured; podcast search is
// disabled in that case while feed-URL subscriptions keep working.
var DefaultClient *client
