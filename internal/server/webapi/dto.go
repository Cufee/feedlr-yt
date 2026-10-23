package webapi

import (
	"fmt"
	"time"

	"github.com/cufee/feedlr-yt/internal/api/sponsorblock"
	shared "github.com/cufee/feedlr-yt/internal/content"
	"github.com/cufee/feedlr-yt/internal/types"
)

type Problem struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
type Error struct {
	Error Problem `json:"error"`
}
type Capabilities struct {
	PodcastSearch bool `json:"podcastSearch"`
	YouTubeSync   bool `json:"youtubeSync"`
	TVSync        bool `json:"tvSync"`
}
type Session struct {
	Authenticated bool         `json:"authenticated"`
	UserID        string       `json:"userId,omitempty"`
	Username      string       `json:"username,omitempty"`
	Capabilities  Capabilities `json:"capabilities"`
}
type Page struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"hasMore"`
	Total   *int `json:"total,omitempty"`
}
type Channel struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ArtworkURL    string `json:"artworkUrl"`
	IsPodcast     bool   `json:"isPodcast"`
	Source        string `json:"source"`
	Subscribed    bool   `json:"subscribed"`
	VideoFilter   string `json:"videoFilter"`
	FeedURL       string `json:"feedUrl,omitempty"`
	FeedUpdatedAt string `json:"feedUpdatedAt,omitempty"`
}
type MediaItem struct {
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	DescriptionHTML string  `json:"descriptionHtml,omitempty"`
	Channel         Channel `json:"channel"`
	ArtworkURL      string  `json:"artworkUrl"`
	PublishedAt     string  `json:"publishedAt"`
	DurationSeconds *int    `json:"durationSeconds"`
	ProgressSeconds int     `json:"progressSeconds"`
	Hidden          bool    `json:"hidden"`
	InWatchLater    bool    `json:"inWatchLater"`
	Completed       bool    `json:"completed"`
	StreamURL       string  `json:"streamUrl,omitempty"`
}
type Playlist struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	Slug              string `json:"slug"`
	VideoCount        int    `json:"videoCount"`
	ProgressPercent   int    `json:"progressPercent"`
	ThumbnailURL      string `json:"thumbnailUrl"`
	System            bool   `json:"system"`
	YouTubePlaylistID string `json:"youtubePlaylistId,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}
type MediaList struct {
	Items []MediaItem `json:"items"`
	Page  Page        `json:"page"`
}
type Feed struct {
	New        []MediaItem `json:"new"`
	Watched    []MediaItem `json:"watched"`
	WatchLater []MediaItem `json:"watchLater"`
	Onboarding bool        `json:"onboarding"`
}
type MediaDetail struct {
	Item            MediaItem                         `json:"item"`
	Authenticated   bool                              `json:"authenticated"`
	ReportProgress  bool                              `json:"reportProgress"`
	Volume          int                               `json:"volume"`
	SkipSegments    []types.SegmentProps              `json:"skipSegments"`
	PodcastSegments types.PodcastSegmentAnalysisProps `json:"podcastSegments"`
	Playlists       []Playlist                        `json:"playlists"`
	MembershipIDs   []string                          `json:"membershipIds"`
}
type Category struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type SegmentSettings struct {
	Enabled             bool       `json:"enabled"`
	Categories          []string   `json:"categories"`
	AvailableCategories []Category `json:"availableCategories"`
}
type YouTubeStatus struct {
	Available    bool   `json:"available"`
	Connected    bool   `json:"connected"`
	Enabled      bool   `json:"enabled"`
	PlaylistID   string `json:"playlistId"`
	LastError    string `json:"lastError"`
	LastSyncedAt string `json:"lastSyncedAt"`
}
type TVStatus struct {
	Available          bool   `json:"available"`
	Connected          bool   `json:"connected"`
	Enabled            bool   `json:"enabled"`
	ConnectionState    string `json:"connectionState"`
	StateReason        string `json:"stateReason"`
	ScreenName         string `json:"screenName"`
	LastError          string `json:"lastError"`
	LastConnectedAt    string `json:"lastConnectedAt"`
	LastEventAt        string `json:"lastEventAt"`
	LastDisconnectAt   string `json:"lastDisconnectAt"`
	LastUserActivityAt string `json:"lastUserActivityAt"`
}
type Passkey struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	CreatedAt string `json:"createdAt"`
}
type Settings struct {
	SponsorBlock    SegmentSettings `json:"sponsorBlock"`
	PodcastSegments SegmentSettings `json:"podcastSegments"`
	YouTubeSync     YouTubeStatus   `json:"youtubeSync"`
	TVSync          TVStatus        `json:"tvSync"`
	Passkeys        []Passkey       `json:"passkeys"`
	Volume          int             `json:"volume"`
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func channelDTO(p types.ChannelProps, subscribed bool) Channel {
	filter := string(p.VideoFilter)
	if filter == "" {
		filter = "all"
	}
	return Channel{ID: p.ID, Title: p.Title, Description: p.Description, ArtworkURL: "/thumb/channel/" + p.ID, IsPodcast: p.IsPodcast, Source: string(p.MediaSource()), Subscribed: subscribed, VideoFilter: filter, FeedUpdatedAt: stamp(p.FeedUpdatedAt)}
}
func mediaDTO(p types.VideoProps) MediaItem {
	kind := "youtube"
	if p.IsPodcast() {
		kind = "podcast"
		p.Channel.IsPodcast = true
	}
	var duration *int
	if p.Duration > 0 {
		n := p.Duration
		duration = &n
	}
	m := MediaItem{ID: p.ID, Kind: kind, Type: string(p.Type), Title: p.Title, Description: p.Description, Channel: channelDTO(p.Channel, false), ArtworkURL: fmt.Sprintf("/thumb/video/%s/sddefault", p.ID), PublishedAt: stamp(p.PublishedAt), DurationSeconds: duration, ProgressSeconds: p.Progress, Hidden: p.Hidden, InWatchLater: p.InWatchLater, Completed: types.IsCompleted(p.Progress, p.Duration), StreamURL: p.StreamURL}
	if p.IsPodcast() {
		m.ArtworkURL = m.Channel.ArtworkURL
		m.DescriptionHTML = shared.SanitizePodcastHTML(p.Description)
	}
	return m
}
func mediaDTOs(items []types.VideoProps) []MediaItem {
	out := make([]MediaItem, 0, len(items))
	for _, p := range items {
		out = append(out, mediaDTO(p))
	}
	return out
}
func playlistDTO(p types.PlaylistProps) Playlist {
	thumb := ""
	if p.ThumbnailVideoID != "" {
		thumb = "/thumb/video/" + p.ThumbnailVideoID + "/sddefault"
	}
	return Playlist{ID: p.ID, Name: p.Name, Description: p.Description, Slug: p.Slug, VideoCount: p.VideoCount, ProgressPercent: p.Progress, ThumbnailURL: thumb, YouTubePlaylistID: p.YouTubePlaylistID, UpdatedAt: stamp(p.UpdatedAt)}
}
func playlistDTOs(items []types.PlaylistProps) []Playlist {
	out := make([]Playlist, 0, len(items))
	for _, p := range items {
		out = append(out, playlistDTO(p))
	}
	return out
}
func categoryDTOs(items []sponsorblock.Category) []Category {
	out := make([]Category, 0, len(items))
	for _, p := range items {
		out = append(out, Category{Value: p.Value, Name: p.Name, Description: p.Description})
	}
	return out
}
